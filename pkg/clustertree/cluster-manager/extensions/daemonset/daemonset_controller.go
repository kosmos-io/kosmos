package daemonset

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	kosmosinformer "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions/kosmos/v1alpha1"
	kosmoslister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils/keys"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
)

var ControllerKind = kosmosv1alpha1.SchemeGroupVersion.WithKind("DaemonSet")

// DaemonSetsController is responsible for synchronizing DaemonSets
type DaemonSetsController struct {
	kosmosClient versioned.Interface

	kubeClient clientset.Interface

	dsLister kosmoslister.DaemonSetLister

	sdsLister kosmoslister.ShadowDaemonSetLister

	clusterLister kosmoslister.ClusterLister

	eventBroadcaster record.EventBroadcaster

	eventRecorder record.EventRecorder

	daemonSetSynced cache.InformerSynced

	shadowDaemonSetSynced cache.InformerSynced

	clusterSynced cache.InformerSynced

	processor lifted.AsyncWorker

	rateLimiterOptions lifted.RateLimitOptions
}

// NewDaemonSetsController returns a new DaemonSetsController
func NewDaemonSetsController(
	shadowDaemonSetInformer kosmosinformer.ShadowDaemonSetInformer,
	daemonSetInformer kosmosinformer.DaemonSetInformer,
	clusterInformer kosmosinformer.ClusterInformer,
	kubeClient clientset.Interface,
	kosmosClient versioned.Interface,
	rateLimiterOptions lifted.RateLimitOptions,
) *DaemonSetsController {
	err := kosmosv1alpha1.Install(scheme.Scheme)
	if err != nil {
		panic(err)
	}
	eventBroadcaster := record.NewBroadcaster()

	dsc := &DaemonSetsController{
		kubeClient:         kubeClient,
		kosmosClient:       kosmosClient,
		dsLister:           daemonSetInformer.Lister(),
		sdsLister:          shadowDaemonSetInformer.Lister(),
		clusterLister:      clusterInformer.Lister(),
		eventBroadcaster:   eventBroadcaster,
		eventRecorder:      eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "daemonset-controller"}),
		rateLimiterOptions: rateLimiterOptions,
	}

	// nolint:errcheck
	shadowDaemonSetInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    dsc.addShadowDaemonSet,
			UpdateFunc: dsc.updateShadowDaemonSet,
			DeleteFunc: dsc.deleteShadowDaemonSet,
		},
	})

	dsc.shadowDaemonSetSynced = shadowDaemonSetInformer.Informer().HasSynced

	// nolint:errcheck
	daemonSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    dsc.addDaemonSet,
		UpdateFunc: dsc.updateDaemonSet,
		DeleteFunc: dsc.deleteDaemonSet,
	})
	dsc.daemonSetSynced = daemonSetInformer.Informer().HasSynced

	// nolint:errcheck
	clusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    dsc.addCluster,
		UpdateFunc: dsc.updateCluster,
		DeleteFunc: dsc.deleteKNode,
	})
	dsc.clusterSynced = clusterInformer.Informer().HasSynced

	return dsc
}

// Run starts the DaemonSetsController
func (dsc *DaemonSetsController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	dsc.eventBroadcaster.StartStructuredLogging(0)
	dsc.eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: dsc.kubeClient.CoreV1().Events("")})
	defer dsc.eventBroadcaster.Shutdown()

	klog.Infof("Starting daemon set controller")
	defer klog.Infof("Shutting down daemon set controller")

	opt := lifted.WorkerOptions{
		Name: "daemon set controller",
		KeyFunc: func(obj interface{}) (lifted.QueueKey, error) {
			return keys.ClusterWideKeyFunc(obj)
		},
		ReconcileFunc:      dsc.syncDaemonSet,
		RateLimiterOptions: dsc.rateLimiterOptions,
	}
	dsc.processor = lifted.NewAsyncWorker(opt)

	if !cache.WaitForNamedCacheSync("kosmos_daemonset_controller", ctx.Done(), dsc.daemonSetSynced, dsc.shadowDaemonSetSynced, dsc.clusterSynced) {
		klog.Errorf("Timed out waiting for caches to sync")
		return
	}
	dsc.processor.Run(workers, ctx.Done())
}

func (dsc *DaemonSetsController) addDaemonSet(obj interface{}) {
	ds := obj.(*kosmosv1alpha1.DaemonSet)
	klog.V(4).Infof("Adding daemon set %s", ds.Name)
	dsc.processor.Enqueue(ds)
}

func (dsc *DaemonSetsController) updateDaemonSet(_, newObj interface{}) {
	newDS := newObj.(*kosmosv1alpha1.DaemonSet)
	klog.V(4).Infof("Updating daemon set %s", newDS.Name)
	dsc.processor.Enqueue(newDS)
}

func (dsc *DaemonSetsController) deleteDaemonSet(obj interface{}) {
	ds := obj.(*kosmosv1alpha1.DaemonSet)
	klog.V(4).Infof("Deleting daemon set %s", ds.Name)
	dsc.processor.Enqueue(ds)
}

func (dsc *DaemonSetsController) processShadowDaemonSet(sds *kosmosv1alpha1.ShadowDaemonSet) {
	// If it has a ControllerRef, that's all that matters.
	if controllerRef := metav1.GetControllerOf(sds); controllerRef != nil {
		ds := dsc.resolveControllerRef(sds.Namespace, controllerRef)
		if ds == nil {
			return
		}
		dsc.processor.Enqueue(ds)
		return
	}
}

func (dsc *DaemonSetsController) addShadowDaemonSet(obj interface{}) {
	sds := obj.(*kosmosv1alpha1.ShadowDaemonSet)
	klog.V(4).Infof("adding shadow daemon set %s", sds.Name)
	dsc.processShadowDaemonSet(sds)
}

func (dsc *DaemonSetsController) updateShadowDaemonSet(_, newObj interface{}) {
	newSDS := newObj.(*kosmosv1alpha1.ShadowDaemonSet)
	klog.V(4).Infof("updating shadow daemon set %s", newSDS.Name)
	dsc.processShadowDaemonSet(newSDS)
}

func (dsc *DaemonSetsController) deleteShadowDaemonSet(obj interface{}) {
	sds := obj.(*kosmosv1alpha1.ShadowDaemonSet)
	klog.V(4).Infof("deleting shadow daemon set %s", sds.Name)
	dsc.processShadowDaemonSet(sds)
}

func (dsc *DaemonSetsController) processCluster() {
	//TODO add should run on node logic
	list, err := dsc.dsLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("failed to list daemon sets: %v", err)
		return
	}
	for _, ds := range list {
		dsc.processor.Enqueue(ds)
	}
}

func (dsc *DaemonSetsController) addCluster(_ interface{}) {
	dsc.processCluster()
}

func (dsc *DaemonSetsController) updateCluster(_ interface{}, _ interface{}) {
	dsc.processCluster()
}

func (dsc *DaemonSetsController) deleteKNode(_ interface{}) {
	dsc.processCluster()
}

func (dsc *DaemonSetsController) syncDaemonSet(key lifted.QueueKey) error {
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Errorf("invalid key type %T", key)
		return fmt.Errorf("invalid key")
	}

	namespace := clusterWideKey.Namespace
	name := clusterWideKey.Name

	ds, err := dsc.dsLister.DaemonSets(namespace).Get(name)

	if apierrors.IsNotFound(err) {
		klog.V(3).Infof("daemon set has been deleted %v", key)
		return nil
	}

	err = dsc.removeOrphanShadowDaemonSet(ds)
	if err != nil {
		klog.Errorf("failed to remove orphan shadow daemon set for daemon set %s err: %v", ds.Name, err)
		return err
	}

	clusterList, err := dsc.clusterLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("couldn't get list of cluster when syncing daemon set %#v: %v", ds, err)
	}

	// sync daemonset
	// sync host shadowDaemonSet
	sdsHost := createShadowDaemonSet(ds, kosmosv1alpha1.RefTypeHost, "")
	err = dsc.createOrUpdate(context.TODO(), sdsHost)
	if err != nil {
		klog.Errorf("failed sync ShadowDaemonSet %s err: %v", sdsHost.DaemonSetSpec, err)
		return err
	}

	// sync member shadowDaemonSet
	for _, cluster := range clusterList {
		if cluster.DeletionTimestamp == nil {
			memberSds := createShadowDaemonSet(ds, kosmosv1alpha1.RefTypeMember, cluster.Name)
			err = dsc.createOrUpdate(context.TODO(), memberSds)
			if err != nil {
				klog.Errorf("failed sync ShadowDaemonSet %s err: %v", memberSds.DaemonSetSpec, err)
				//klog.Errorf("failed sync ShadowDaemonSet %s err: %v", sdsHost.DaemonSetSpec, err)
				//return err
			}
		}
	}

	// update status
	return dsc.updateStatus(context.TODO(), ds)
}

func (dsc *DaemonSetsController) createOrUpdate(ctx context.Context, ds *kosmosv1alpha1.ShadowDaemonSet) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		exist, err := dsc.kosmosClient.KosmosV1alpha1().ShadowDaemonSets(ds.Namespace).Get(ctx, ds.Name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			// create new
			_, err := dsc.kosmosClient.KosmosV1alpha1().ShadowDaemonSets(ds.Namespace).Create(ctx, ds, metav1.CreateOptions{})
			if err != nil {
				klog.Errorf("Failed create ShadowDaemonSet %s err: %v", ds.Name, err)
				return err
			}
			return nil
		}
		// update exist
		desired := exist.DeepCopy()
		desired.DaemonSetSpec = ds.DaemonSetSpec
		desired.SetLabels(ds.Labels)
		desired.SetAnnotations(ds.Annotations)
		_, err = dsc.kosmosClient.KosmosV1alpha1().ShadowDaemonSets(ds.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("Failed update ShadowDaemonSet %s err: %v", ds.Name, err)
			return err
		}
		return nil
	})
	if err != nil {
		klog.Errorf("Failed create or update ShadowDaemonSet %s err: %v", ds.Name, err)
		return err
	}
	return nil
}

func (dsc *DaemonSetsController) updateStatus(ctx context.Context, ds *kosmosv1alpha1.DaemonSet) error {
	sds, err := listAllShadowDaemonSet(dsc.sdsLister, ds)
	if err != nil {
		klog.Errorf("Failed list ShadowDaemonSet for %s err: %v", ds.Name, err)
		return err
	}
	desiredNumberScheduled := int32(0)
	currentNumberScheduled := int32(0)
	numberMisscheduled := int32(0)
	numberReady := int32(0)
	updatedNumberScheduled := int32(0)
	numberAvailable := int32(0)
	numberUnavailable := int32(0)

	for _, s := range sds {
		desiredNumberScheduled = desiredNumberScheduled + s.Status.DesiredNumberScheduled
		currentNumberScheduled = currentNumberScheduled + s.Status.CurrentNumberScheduled
		numberMisscheduled = numberMisscheduled + s.Status.NumberMisscheduled
		numberReady = numberReady + s.Status.NumberReady
		updatedNumberScheduled = updatedNumberScheduled + s.Status.UpdatedNumberScheduled
		numberAvailable = numberAvailable + s.Status.NumberAvailable
		numberUnavailable = numberUnavailable + s.Status.NumberUnavailable
	}
	toUpdate := ds.DeepCopy()
	toUpdate.Status.DesiredNumberScheduled = desiredNumberScheduled
	toUpdate.Status.CurrentNumberScheduled = currentNumberScheduled
	toUpdate.Status.NumberMisscheduled = numberMisscheduled
	toUpdate.Status.NumberReady = numberReady
	toUpdate.Status.UpdatedNumberScheduled = updatedNumberScheduled
	toUpdate.Status.NumberAvailable = numberAvailable
	toUpdate.Status.NumberUnavailable = numberUnavailable

	if _, updateErr := dsc.kosmosClient.KosmosV1alpha1().DaemonSets(ds.Namespace).UpdateStatus(ctx, toUpdate, metav1.UpdateOptions{}); updateErr != nil {
		klog.Errorf("Failed update DaemonSet %s status err: %v", ds.Name, updateErr)
		return updateErr
	}
	return nil
}

func (dsc *DaemonSetsController) resolveControllerRef(namespace string, ref *metav1.OwnerReference) *kosmosv1alpha1.DaemonSet {
	if ref.Kind != "DaemonSet" {
		return nil
	}
	ds, err := dsc.dsLister.DaemonSets(namespace).Get(ref.Name)
	if err != nil {
		klog.Errorf("Failed get DaemonSet %s err: %v", ref.Name, err)
		return nil
	}
	return ds
}

func (dsc *DaemonSetsController) removeOrphanShadowDaemonSet(ds *kosmosv1alpha1.DaemonSet) error {
	allSds, err := listAllShadowDaemonSet(dsc.sdsLister, ds)
	if err != nil {
		klog.Errorf("Failed list ShadowDaemonSet for %s err: %v", ds.Name, err)
		return err
	}
	clusterList, err := dsc.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("couldn't get list of clusters when syncing daemon set %#v: %v", ds, err)
		return err
	}
	clusterSet := make(map[string]interface{})
	for _, cluster := range clusterList {
		clusterSet[cluster.Name] = struct{}{}
	}

	for _, s := range allSds {
		if s.RefType == kosmosv1alpha1.RefTypeHost {
			continue
		}
		if _, ok := clusterSet[s.Cluster]; !ok {
			err = dsc.kosmosClient.KosmosV1alpha1().ShadowDaemonSets(s.Namespace).Delete(context.TODO(), s.Name,
				metav1.DeleteOptions{})
			if err != nil {
				klog.Errorf("Failed delete ShadowDaemonSet %s err: %v", s.Name, err)
				return err
			}
		}
	}
	return nil
}

func listAllShadowDaemonSet(lister kosmoslister.ShadowDaemonSetLister, ds *kosmosv1alpha1.DaemonSet) ([]*kosmosv1alpha1.ShadowDaemonSet, error) {
	list, err := lister.ShadowDaemonSets(ds.Namespace).List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed list ShadowDaemonSet for %s err: %v", ds.Name, err)
		return nil, err
	}
	var sds []*kosmosv1alpha1.ShadowDaemonSet
	for _, s := range list {
		if s.OwnerReferences != nil {
			for _, ref := range s.OwnerReferences {
				if ref.UID == ds.UID {
					sds = append(sds, s)
				}
			}
		}
	}
	return sds, nil
}

func createShadowDaemonSet(ds *kosmosv1alpha1.DaemonSet, refType kosmosv1alpha1.RefType, cluster string) *kosmosv1alpha1.ShadowDaemonSet {
	suffix := "-host"
	if refType != kosmosv1alpha1.RefTypeHost {
		suffix = "-" + cluster
	}
	var sds *kosmosv1alpha1.ShadowDaemonSet
	if cluster != "" {
		sds = &kosmosv1alpha1.ShadowDaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Annotations:     ds.Annotations,
				Labels:          ds.Labels,
				Namespace:       ds.Namespace,
				Name:            ds.Name + suffix,
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(ds, ControllerKind)},
			},
			RefType:       refType,
			Cluster:       cluster,
			DaemonSetSpec: ds.Spec,
		}
	} else {
		sds = &kosmosv1alpha1.ShadowDaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Annotations:     ds.Annotations,
				Labels:          ds.Labels,
				Namespace:       ds.Namespace,
				Name:            ds.Name + suffix,
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(ds, ControllerKind)},
			},
			RefType:       refType,
			DaemonSetSpec: ds.Spec,
		}
	}

	return sds
}
