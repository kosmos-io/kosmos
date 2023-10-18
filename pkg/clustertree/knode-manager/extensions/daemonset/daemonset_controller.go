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
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/flags"
	"github.com/kosmos.io/kosmos/pkg/utils/keys"
)

var ControllerKind = kosmosv1alpha1.SchemeGroupVersion.WithKind("DaemonSet")

// DaemonSetsController is responsible for synchronizing DaemonSets
type DaemonSetsController struct {
	kosmosClient versioned.Interface

	kubeClient clientset.Interface

	dsLister kosmoslister.DaemonSetLister

	sdsLister kosmoslister.ShadowDaemonSetLister

	kNodeLister kosmoslister.KnodeLister

	eventBroadcaster record.EventBroadcaster

	eventRecorder record.EventRecorder

	daemonSetSynced cache.InformerSynced

	shadowDaemonSetSynced cache.InformerSynced

	kNodeSynced cache.InformerSynced

	processor utils.AsyncWorker

	rateLimiterOptions flags.Options
}

// NewDaemonSetsController returns a new DaemonSetsController
func NewDaemonSetsController(
	shadowDaemonSetInformer kosmosinformer.ShadowDaemonSetInformer,
	daemonSetInformer kosmosinformer.DaemonSetInformer,
	kNodeInformer kosmosinformer.KnodeInformer,
	kubeClient clientset.Interface,
	kosmosClient versioned.Interface,
	dsLister kosmoslister.DaemonSetLister,
	sdsLister kosmoslister.ShadowDaemonSetLister,
	kNodeLister kosmoslister.KnodeLister,
	rateLimiterOptions flags.Options,
) *DaemonSetsController {
	err := kosmosv1alpha1.Install(scheme.Scheme)
	if err != nil {
		panic(err)
	}
	eventBroadcaster := record.NewBroadcaster()

	dsc := &DaemonSetsController{
		kubeClient:         kubeClient,
		kosmosClient:       kosmosClient,
		dsLister:           dsLister,
		sdsLister:          sdsLister,
		kNodeLister:        kNodeLister,
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
	kNodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    dsc.addKNode,
		DeleteFunc: dsc.deleteKNode,
	})
	dsc.kNodeSynced = kNodeInformer.Informer().HasSynced

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

	opt := utils.Options{
		Name: "daemon set controller",
		KeyFunc: func(obj interface{}) (utils.QueueKey, error) {
			return keys.ClusterWideKeyFunc(obj)
		},
		ReconcileFunc:      dsc.syncDaemonSet,
		RateLimiterOptions: dsc.rateLimiterOptions,
	}
	dsc.processor = utils.NewAsyncWorker(opt)

	if !cache.WaitForNamedCacheSync("kosmos_daemonset_controller", ctx.Done(), dsc.daemonSetSynced, dsc.shadowDaemonSetSynced, dsc.kNodeSynced) {
		klog.V(2).Infof("Timed out waiting for caches to sync")
		return
	}
	dsc.processor.Run(workers, ctx.Done())

	<-ctx.Done()
}

func (dsc *DaemonSetsController) addDaemonSet(obj interface{}) {
	ds := obj.(*kosmosv1alpha1.DaemonSet)
	klog.V(4).Infof("Adding daemon set %s", ds.Name)
	dsc.processor.Enqueue(ds)
}

func (dsc *DaemonSetsController) updateDaemonSet(oldObj, newObj interface{}) {
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

func (dsc *DaemonSetsController) updateShadowDaemonSet(oldObj, newObj interface{}) {
	newSDS := newObj.(*kosmosv1alpha1.ShadowDaemonSet)
	klog.V(4).Infof("updating shadow daemon set %s", newSDS.Name)
	dsc.processShadowDaemonSet(newSDS)
}

func (dsc *DaemonSetsController) deleteShadowDaemonSet(obj interface{}) {
	sds := obj.(*kosmosv1alpha1.ShadowDaemonSet)
	klog.V(4).Infof("deleting shadow daemon set %s", sds.Name)
	dsc.processShadowDaemonSet(sds)
}

func (dsc *DaemonSetsController) processKNode(knode *kosmosv1alpha1.Knode) {
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

func (dsc *DaemonSetsController) addKNode(obj interface{}) {
	kNode := obj.(*kosmosv1alpha1.Knode)
	klog.V(4).Infof("adding knode %s", kNode.Name)
	dsc.processKNode(kNode)
}

func (dsc *DaemonSetsController) deleteKNode(obj interface{}) {
	kNode := obj.(*kosmosv1alpha1.Knode)
	klog.V(4).Infof("deleting knode %s", kNode.Name)
	dsc.processKNode(kNode)
}

func (dsc *DaemonSetsController) syncDaemonSet(key utils.QueueKey) error {
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.V(2).Infof("invalid key type %T", key)
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
		klog.V(2).Infof("failed to remove orphan shadow daemon set for daemon set %s err: %v", ds.Name, err)
		return err
	}

	kNodeList, err := dsc.kNodeLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("couldn't get list of knodes when syncing daemon set %#v: %v", ds, err)
	}
	// sync daemonset
	// sync host shadowDaemonSet
	sdsHost := createShadowDaemonSet(ds, kosmosv1alpha1.RefTypeHost, "")
	err = dsc.createOrUpdate(context.TODO(), sdsHost)
	if err != nil {
		klog.V(2).Infof("failed sync ShadowDaemonSet %s err: %v", sdsHost.DaemonSetSpec, err)
		return err
	}

	// sync member shadowDaemonSet
	for _, knode := range kNodeList {
		if knode.DeletionTimestamp == nil {
			memberSds := createShadowDaemonSet(ds, kosmosv1alpha1.RefTypeMember, knode.Name)
			err = dsc.createOrUpdate(context.TODO(), memberSds)
			if err != nil {
				klog.V(2).Infof("failed sync ShadowDaemonSet %s err: %v", sdsHost.DaemonSetSpec, err)
				return err
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
				klog.V(2).Infof("Failed create ShadowDaemonSet %s err: %v", ds.Name, err)
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
			klog.V(2).Infof("Failed update ShadowDaemonSet %s err: %v", ds.Name, err)
			return err
		}
		return nil
	})
	if err != nil {
		klog.V(2).Infof("Failed create or update ShadowDaemonSet %s err: %v", ds.Name, err)
		return err
	}
	return nil
}

func (dsc *DaemonSetsController) updateStatus(ctx context.Context, ds *kosmosv1alpha1.DaemonSet) error {
	sds, err := listAllShadowDaemonSet(dsc.sdsLister, ds)
	if err != nil {
		klog.V(2).Infof("Failed list ShadowDaemonSet for %s err: %v", ds.Name, err)
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
		klog.V(2).Infof("Failed update DaemonSet %s status err: %v", ds.Name, updateErr)
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
		klog.V(2).Infof("Failed get DaemonSet %s err: %v", ref.Name, err)
		return nil
	}
	return ds
}

func (dsc *DaemonSetsController) removeOrphanShadowDaemonSet(ds *kosmosv1alpha1.DaemonSet) error {
	allSds, err := listAllShadowDaemonSet(dsc.sdsLister, ds)
	if err != nil {
		klog.V(2).Infof("Failed list ShadowDaemonSet for %s err: %v", ds.Name, err)
		return err
	}
	kNodeList, err := dsc.kNodeLister.List(labels.Everything())
	if err != nil {
		klog.V(2).Infof("couldn't get list of knodes when syncing daemon set %#v: %v", ds, err)
		return err
	}
	knodeSet := make(map[string]interface{})
	for _, kNode := range kNodeList {
		knodeSet[kNode.Name] = struct{}{}
	}

	for _, s := range allSds {
		if s.RefType == kosmosv1alpha1.RefTypeHost {
			continue
		}
		if _, ok := knodeSet[s.Knode]; !ok {
			err = dsc.kosmosClient.KosmosV1alpha1().ShadowDaemonSets(s.Namespace).Delete(context.TODO(), s.Name,
				metav1.DeleteOptions{})
			if err != nil {
				klog.V(2).Infof("Failed delete ShadowDaemonSet %s err: %v", s.Name, err)
				return err
			}
		}
	}
	return nil
}

func listAllShadowDaemonSet(lister kosmoslister.ShadowDaemonSetLister, ds *kosmosv1alpha1.DaemonSet) ([]*kosmosv1alpha1.ShadowDaemonSet, error) {
	list, err := lister.ShadowDaemonSets(ds.Namespace).List(labels.Everything())
	if err != nil {
		klog.V(2).Infof("Failed list ShadowDaemonSet for %s err: %v", ds.Name, err)
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

func createShadowDaemonSet(ds *kosmosv1alpha1.DaemonSet, refType kosmosv1alpha1.RefType, nodeName string) *kosmosv1alpha1.ShadowDaemonSet {
	suffix := "-host"
	if refType != kosmosv1alpha1.RefTypeHost {
		suffix = "-" + nodeName
	}
	var sds *kosmosv1alpha1.ShadowDaemonSet
	if nodeName != "" {
		sds = &kosmosv1alpha1.ShadowDaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Annotations:     ds.Annotations,
				Labels:          ds.Labels,
				Namespace:       ds.Namespace,
				Name:            ds.Name + suffix,
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(ds, ControllerKind)},
			},
			RefType:       refType,
			Knode:         nodeName,
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
