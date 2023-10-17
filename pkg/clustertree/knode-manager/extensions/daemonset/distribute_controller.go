package daemonset

import (
	"context"
	"fmt"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	informer "k8s.io/client-go/informers"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	kosmosinformer "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions/kosmos/v1alpha1"
	kosmoslister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/flags"
	"github.com/kosmos.io/kosmos/pkg/utils/keys"
)

// DistributeController is responsible for propagating the shadow daemon set to the member cluster
type DistributeController struct {
	kosmosClient versioned.Interface

	sdsLister kosmoslister.ShadowDaemonSetLister

	kNodeLister kosmoslister.KnodeLister

	shadowDaemonSetSynced cache.InformerSynced

	kNodeSynced cache.InformerSynced

	knodeProcessor utils.AsyncWorker

	shadowDaemonSetProcessor utils.AsyncWorker

	knodeDaemonSetManagerMap map[string]*KNodeDaemonSetManager

	rateLimiterOptions flags.Options

	lock sync.RWMutex
}

func NewDistributeController(
	kosmosClient versioned.Interface,
	sdsInformer kosmosinformer.ShadowDaemonSetInformer,
	knodeInformer kosmosinformer.KnodeInformer,
	rateLimiterOptions flags.Options,
) *DistributeController {
	dc := &DistributeController{
		kosmosClient:             kosmosClient,
		sdsLister:                sdsInformer.Lister(),
		kNodeLister:              knodeInformer.Lister(),
		shadowDaemonSetSynced:    sdsInformer.Informer().HasSynced,
		kNodeSynced:              knodeInformer.Informer().HasSynced,
		knodeDaemonSetManagerMap: make(map[string]*KNodeDaemonSetManager),
		rateLimiterOptions:       rateLimiterOptions,
	}

	// nolint:errcheck
	knodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    dc.addKNode,
		UpdateFunc: dc.updateKNode,
		DeleteFunc: dc.deleteKNode,
	})
	// nolint:errcheck
	sdsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			sds := obj.(*v1alpha1.ShadowDaemonSet)
			return sds.RefType == v1alpha1.RefTypeMember
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    dc.addShadowDaemonSet,
			UpdateFunc: dc.updateShadowDaemonSet,
			DeleteFunc: dc.deleteShadowDaemonSet,
		},
	})
	return dc
}

func (dc *DistributeController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.Infof("Starting distribute controller")
	defer klog.Infof("Shutting down distribute controller")

	knodeOpt := utils.Options{
		Name: "distribute controller: KNode",
		KeyFunc: func(obj interface{}) (utils.QueueKey, error) {
			return keys.ClusterWideKeyFunc(obj)
		},
		ReconcileFunc:      dc.syncKNode,
		RateLimiterOptions: dc.rateLimiterOptions,
	}
	dc.knodeProcessor = utils.NewAsyncWorker(knodeOpt)

	sdsOpt := utils.Options{
		Name: "distribute controller: ShadowDaemonSet",
		KeyFunc: func(obj interface{}) (utils.QueueKey, error) {
			return keys.ClusterWideKeyFunc(obj)
		},
		ReconcileFunc:      dc.syncShadowDaemonSet,
		RateLimiterOptions: dc.rateLimiterOptions,
	}
	dc.shadowDaemonSetProcessor = utils.NewAsyncWorker(sdsOpt)

	if !cache.WaitForNamedCacheSync("host_daemon_controller", ctx.Done(), dc.shadowDaemonSetSynced, dc.kNodeSynced) {
		klog.V(2).Infof("Timed out waiting for caches to sync")
		return
	}

	dc.knodeProcessor.Run(workers, ctx.Done())
	dc.shadowDaemonSetProcessor.Run(workers, ctx.Done())
	<-ctx.Done()
}

func (dc *DistributeController) syncKNode(key utils.QueueKey) error {
	dc.lock.Lock()
	defer dc.lock.Unlock()
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.V(2).Infof("invalid key type %T", key)
		return fmt.Errorf("invalid key")
	}
	name := clusterWideKey.Name
	knode, err := dc.kNodeLister.Get(name)

	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(3).Infof("knode has been deleted %v", key)
			return nil
		}
		return err
	}

	manager, ok := dc.knodeDaemonSetManagerMap[knode.Name]
	if !ok {
		config, err := clientcmd.RESTConfigFromKubeConfig(knode.Spec.Kubeconfig)
		if err != nil {
			klog.V(2).Infof("failed to create rest config for knode %s: %v", knode.Name, err)
			return err
		}

		kubeClient, err := kubernetes.NewForConfig(config)
		if err != nil {
			klog.V(2).Infof("failed to create kube client for knode %s: %v", knode.Name, err)
		}

		kubeFactory := informers.NewSharedInformerFactory(kubeClient, 0)

		dsInformer := kubeFactory.Apps().V1().DaemonSets()
		// nolint:errcheck
		dsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				ds, ok := obj.(*appsv1.DaemonSet)
				if !ok {
					return false
				}
				if ds.Labels[ManagedLabel] == "" {
					return true
				}
				return false
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    dc.addDaemonSet,
				DeleteFunc: dc.deleteDaemonSet,
				UpdateFunc: dc.updateDaemonSet,
			},
		})

		daemonsetSynced := dsInformer.Informer().HasSynced()
		manager = NewKNodeDaemonSetManager(
			kubeClient,
			dsInformer,
			kubeFactory,
			daemonsetSynced,
		)
		dc.knodeDaemonSetManagerMap[knode.Name] = manager
		manager.Start()
	}

	if knode.DeletionTimestamp != nil {
		list, error := manager.dsLister.List(labels.Set{ManagedLabel: ""}.AsSelector())
		if error != nil {
			klog.V(2).Infof("failed to list daemonsets from knode %s: %v", knode.Name, error)
			return error
		}
		for i := range list {
			ds := list[i]
			error := manager.kubeClient.AppsV1().DaemonSets(ds.Namespace).Delete(context.Background(), ds.Name, metav1.DeleteOptions{})
			if err != nil {
				klog.V(2).Infof("failed to delete daemonset %s/%s from knode %s: %v", ds.Namespace, ds.Name, knode.Name, error)
				return error
			}
		}
		err = dc.removeKNodeFinalizer(knode)
		if err != nil {
			return err
		}
		manager.Stop()
		delete(dc.knodeDaemonSetManagerMap, knode.Name)
		return err
	}
	return dc.ensureKNodeFinalizer(knode)
}

func (dc *DistributeController) syncShadowDaemonSet(key utils.QueueKey) error {
	dc.lock.RLock()
	defer dc.lock.RUnlock()
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.V(2).Infof("invalid key type %T", key)
		return fmt.Errorf("invalid key")
	}

	namespace := clusterWideKey.Namespace
	name := clusterWideKey.Name

	sds, err := dc.sdsLister.ShadowDaemonSets(namespace).Get(name)

	if apierrors.IsNotFound(err) {
		klog.V(2).Infof("daemon set has been deleted %v", key)
		return nil
	}

	knode, err := dc.kNodeLister.Get(sds.Knode)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.V(2).Infof("failed to get knode %s: %v", sds.Knode, err)
		return err
	}
	// when knode is deleting or not found, skip sync
	if knode == nil || knode.DeletionTimestamp != nil {
		return dc.removeShadowDaemonSetFinalizer(sds)
	}

	manager, ok := dc.knodeDaemonSetManagerMap[sds.Knode]
	if !ok {
		msg := fmt.Sprintf("no manager found for knode %s", sds.Knode)
		klog.V(2).Info(msg)
		return fmt.Errorf(msg)
	}

	if sds.DeletionTimestamp != nil {
		err := manager.kubeClient.AppsV1().DaemonSets(sds.Namespace).Delete(context.Background(), sds.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			klog.V(2).Infof("failed to delete daemonset %s/%s from knode %s: %v", sds.Namespace, sds.Name, sds.Knode, err)
			return err
		}
		return dc.removeShadowDaemonSetFinalizer(sds)
	}

	sds, err = dc.ensureShadowDaemonSetFinalizer(sds)
	if err != nil {
		klog.V(2).Infof("failed to ensure finalizer for shadow daemonset %s/%s: %v", namespace, name, err)
		return err
	}
	copy := sds.DeepCopy()

	err = manager.tryCreateOrUpdateDaemonset(sds)
	if err != nil {
		klog.V(2).Infof("failed to create or update daemonset %s/%s: %v", namespace, name, err)
		return err
	}

	ds, error := manager.dsLister.DaemonSets(sds.Namespace).Get(sds.Name)

	if error != nil {
		klog.V(2).Infof("failed to get daemonset %s/%s: %v", namespace, name, error)
		return error
	}

	error = dc.updateStatus(copy, ds)

	if error != nil {
		klog.V(2).Infof("failed to update status for shadow daemonset %s/%s: %v", namespace, name, error)
		return error
	}
	return nil
}

func (dc *DistributeController) ensureShadowDaemonSetFinalizer(sds *v1alpha1.ShadowDaemonSet) (*v1alpha1.ShadowDaemonSet, error) {
	if controllerutil.ContainsFinalizer(sds, DistributeControllerFinalizer) {
		return sds, nil
	}

	controllerutil.AddFinalizer(sds, DistributeControllerFinalizer)
	sds, err := dc.kosmosClient.KosmosV1alpha1().ShadowDaemonSets(sds.Namespace).Update(context.TODO(), sds, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("sds %s failed add finalizer: %v", sds.Name, err)
		return nil, err
	}

	return sds, nil
}

func (dc *DistributeController) removeShadowDaemonSetFinalizer(sds *v1alpha1.ShadowDaemonSet) error {
	if !controllerutil.ContainsFinalizer(sds, DistributeControllerFinalizer) {
		return nil
	}
	controllerutil.RemoveFinalizer(sds, DistributeControllerFinalizer)
	_, err := dc.kosmosClient.KosmosV1alpha1().ShadowDaemonSets(sds.GetNamespace()).Update(context.TODO(), sds, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("sds %s failed remove finalizer: %v", sds.Name, err)
		return err
	}
	return nil
}

func (dc *DistributeController) ensureKNodeFinalizer(knode *v1alpha1.Knode) error {
	if controllerutil.ContainsFinalizer(knode, DistributeControllerFinalizer) {
		return nil
	}

	controllerutil.AddFinalizer(knode, DistributeControllerFinalizer)
	_, err := dc.kosmosClient.KosmosV1alpha1().Knodes().Update(context.TODO(), knode, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("knode %s failed add finalizer: %v", knode.Name, err)
		return err
	}

	return nil
}

func (dc *DistributeController) removeKNodeFinalizer(knode *v1alpha1.Knode) error {
	if !controllerutil.ContainsFinalizer(knode, DistributeControllerFinalizer) {
		return nil
	}
	controllerutil.RemoveFinalizer(knode, DistributeControllerFinalizer)
	_, err := dc.kosmosClient.KosmosV1alpha1().Knodes().Update(context.TODO(), knode, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("knode %s failed remove finalizer: %v", knode.Name, err)
		return err
	}
	return nil
}

func (dc *DistributeController) updateStatus(sds *v1alpha1.ShadowDaemonSet, ds *appsv1.DaemonSet) error {
	sds.Status.CurrentNumberScheduled = ds.Status.CurrentNumberScheduled
	sds.Status.NumberMisscheduled = ds.Status.NumberMisscheduled
	sds.Status.DesiredNumberScheduled = ds.Status.DesiredNumberScheduled
	sds.Status.NumberReady = ds.Status.NumberReady
	sds.Status.ObservedGeneration = ds.Status.ObservedGeneration
	sds.Status.UpdatedNumberScheduled = ds.Status.UpdatedNumberScheduled
	sds.Status.NumberAvailable = ds.Status.NumberAvailable
	sds.Status.NumberUnavailable = ds.Status.NumberUnavailable
	sds.Status.CollisionCount = ds.Status.CollisionCount
	sds.Status.Conditions = ds.Status.Conditions
	_, error := dc.kosmosClient.KosmosV1alpha1().ShadowDaemonSets(sds.Namespace).UpdateStatus(context.Background(), sds, metav1.UpdateOptions{})
	return error
}

func (dc *DistributeController) addKNode(obj interface{}) {
	ds := obj.(*v1alpha1.Knode)
	klog.V(4).Infof("Adding daemon set %s", ds.Name)
	dc.knodeProcessor.Enqueue(ds)
}

func (dc *DistributeController) updateKNode(oldObj, newObj interface{}) {
	newDS := newObj.(*v1alpha1.Knode)
	klog.V(4).Infof("Updating daemon set %s", newDS.Name)
	dc.knodeProcessor.Enqueue(newDS)
}

func (dc *DistributeController) deleteKNode(obj interface{}) {
	ds := obj.(*v1alpha1.Knode)
	dc.knodeProcessor.Enqueue(ds)
}

func (dc *DistributeController) addDaemonSet(obj interface{}) {
	ds := obj.(*appsv1.DaemonSet)
	dc.shadowDaemonSetProcessor.Enqueue(ds)
}

func (dc *DistributeController) updateDaemonSet(oldObj, newObj interface{}) {
	newDS := newObj.(*appsv1.DaemonSet)
	dc.shadowDaemonSetProcessor.Enqueue(newDS)
}

func (dc *DistributeController) deleteDaemonSet(obj interface{}) {
	ds := obj.(*appsv1.DaemonSet)
	dc.shadowDaemonSetProcessor.Enqueue(ds)
}

func (dc *DistributeController) addShadowDaemonSet(obj interface{}) {
	ds := obj.(*v1alpha1.ShadowDaemonSet)
	klog.V(4).Infof("Adding daemon set %s", ds.Name)
	dc.shadowDaemonSetProcessor.Enqueue(ds)
}

func (dc *DistributeController) updateShadowDaemonSet(oldObj, newObj interface{}) {
	newDS := newObj.(*v1alpha1.ShadowDaemonSet)
	klog.V(4).Infof("Updating daemon set %s", newDS.Name)
	dc.shadowDaemonSetProcessor.Enqueue(newDS)
}

func (dc *DistributeController) deleteShadowDaemonSet(obj interface{}) {
	ds := obj.(*v1alpha1.ShadowDaemonSet)
	klog.V(4).Infof("Deleting daemon set %s", ds.Name)
	dc.shadowDaemonSetProcessor.Enqueue(ds)
}

type KNodeDaemonSetManager struct {
	kubeClient clientset.Interface

	dsLister appslisters.DaemonSetLister

	factory informer.SharedInformerFactory

	version map[string]string

	lock sync.RWMutex

	ctx context.Context

	cancelFun context.CancelFunc

	daemonSetSynced cache.InformerSynced
}

func (km *KNodeDaemonSetManager) Start() {
	km.factory.Start(km.ctx.Done())
	if !cache.WaitForNamedCacheSync("km_manager", km.ctx.Done(), km.daemonSetSynced) {
		klog.Errorf("failed to wait for daemonSet caches to sync")
		return
	}
}

func (km *KNodeDaemonSetManager) Stop() {
	if km.cancelFun != nil {
		km.cancelFun()
	}
}

func (km *KNodeDaemonSetManager) tryCreateOrUpdateDaemonset(sds *v1alpha1.ShadowDaemonSet) error {
	err := km.ensureNameSpace(sds.Namespace)
	if err != nil {
		klog.V(4).Infof("ensure namespace %s failed: %v", sds.Namespace, err)
		return err
	}

	ds, error := km.dsLister.DaemonSets(sds.Namespace).Get(sds.Name)
	copyDs := ds.DeepCopy()
	if error != nil {
		if apierrors.IsNotFound(error) {
			newDaemonSet := &appsv1.DaemonSet{}
			newDaemonSet.Spec.Template = sds.DaemonSetSpec.Template
			newDaemonSet.Spec.Selector = sds.DaemonSetSpec.Selector
			newDaemonSet.Spec.UpdateStrategy = sds.DaemonSetSpec.UpdateStrategy
			newDaemonSet.Spec.MinReadySeconds = sds.DaemonSetSpec.MinReadySeconds
			newDaemonSet.Spec.RevisionHistoryLimit = sds.DaemonSetSpec.RevisionHistoryLimit
			newDaemonSet.Name = sds.Name
			newDaemonSet.Namespace = sds.Namespace
			newDaemonSet.Labels = sds.Labels
			newDaemonSet.Annotations = sds.Annotations
			newDaemonSet.Labels = labels.Set{ManagedLabel: ""}
			newDs, error := km.kubeClient.AppsV1().DaemonSets(sds.Namespace).Create(context.Background(), newDaemonSet, metav1.CreateOptions{})
			if error != nil {
				klog.V(2).Infof("failed to create daemonset %s/%s: %v", sds.Namespace, sds.Name, error)
				return error
			}
			km.updateVersion(newDs)
			return nil
		} else {
			klog.V(2).Infof("failed to get daemonset %s/%s: %v", sds.Namespace, sds.Name, error)
			return error
		}
	}

	if copyDs.ResourceVersion == km.version[key(sds.ObjectMeta)] {
		return nil
	}

	copyDs.Spec.Template = sds.DaemonSetSpec.Template
	copyDs.Spec.Selector = sds.DaemonSetSpec.Selector
	copyDs.Spec.UpdateStrategy = sds.DaemonSetSpec.UpdateStrategy
	copyDs.Spec.MinReadySeconds = sds.DaemonSetSpec.MinReadySeconds
	copyDs.Spec.RevisionHistoryLimit = sds.DaemonSetSpec.RevisionHistoryLimit

	for k, v := range sds.Labels {
		copyDs.Labels[k] = v
	}
	copyDs.Labels[ManagedLabel] = ""

	for k, v := range sds.Annotations {
		// TODO delete  annotations which add by controller
		copyDs.Annotations[k] = v
	}

	updated, error := km.kubeClient.AppsV1().DaemonSets(sds.Namespace).Update(context.Background(), copyDs, metav1.UpdateOptions{})
	if error != nil {
		klog.V(2).Infof("failed to update daemonset %s/%s: %v", sds.Namespace, sds.Name, error)
		return error
	}
	km.updateVersion(updated)
	return nil
}

func (km *KNodeDaemonSetManager) ensureNameSpace(namespace string) error {
	ns := &corev1.Namespace{}
	ns.Name = namespace
	_, err := km.kubeClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		klog.V(2).Infof("failed to create namespace %s: %v", namespace, err)
		return err
	}

	return nil
}

func (km *KNodeDaemonSetManager) updateVersion(ds *appsv1.DaemonSet) {
	km.lock.Lock()
	defer km.lock.Unlock()
	km.version[key(ds.ObjectMeta)] = ds.ResourceVersion
}

func NewKNodeDaemonSetManager(client *clientset.Clientset, dsInformer appsinformers.DaemonSetInformer, factory informer.SharedInformerFactory, synced bool) *KNodeDaemonSetManager {
	ctx := context.TODO()
	ctx, cancel := context.WithCancel(ctx)
	return &KNodeDaemonSetManager{
		kubeClient:      client,
		dsLister:        dsInformer.Lister(),
		factory:         factory,
		ctx:             ctx,
		cancelFun:       cancel,
		version:         map[string]string{},
		daemonSetSynced: dsInformer.Informer().HasSynced,
	}
}

func key(obj metav1.ObjectMeta) string {
	return obj.Namespace + "/" + obj.Name
}
