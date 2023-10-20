package daemonset

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	kosmosinformer "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions/kosmos/v1alpha1"
	kosmoslister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/flags"
	"github.com/kosmos.io/kosmos/pkg/utils/keys"
)

type DaemonSetsMirrorController struct {
	kosmosClient versioned.Interface

	kubeClient clientset.Interface

	dsLister appslisters.DaemonSetLister

	kdsLister kosmoslister.DaemonSetLister

	eventBroadcaster record.EventBroadcaster

	eventRecorder record.EventRecorder

	daemonSetSynced cache.InformerSynced

	kosmosDaemonSetSynced cache.InformerSynced

	processor utils.AsyncWorker

	rateLimiterOptions flags.Options
}

func NewDaemonSetsMirrorController(
	kosmosClient versioned.Interface,
	kubeClient clientset.Interface,
	kdsInformer kosmosinformer.DaemonSetInformer,
	dsInformer appsinformers.DaemonSetInformer,
	rateLimiterOptions flags.Options,
) *DaemonSetsMirrorController {
	err := kosmosv1alpha1.Install(scheme.Scheme)
	if err != nil {
		panic(err)
	}
	eventBroadcaster := record.NewBroadcaster()
	dmc := &DaemonSetsMirrorController{
		kubeClient:         kubeClient,
		kosmosClient:       kosmosClient,
		dsLister:           dsInformer.Lister(),
		kdsLister:          kdsInformer.Lister(),
		eventBroadcaster:   eventBroadcaster,
		eventRecorder:      eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "daemonset-mirror-controller"}),
		rateLimiterOptions: rateLimiterOptions,
	}
	dmc.daemonSetSynced = dsInformer.Informer().HasSynced
	dmc.kosmosDaemonSetSynced = kdsInformer.Informer().HasSynced
	// nolint:errcheck
	dsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			ds, ok := obj.(*appsv1.DaemonSet)
			if !ok {
				return false
			}
			return ds.Annotations[MirrorAnnotation] == "true"
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    dmc.AddDaemonSet,
			UpdateFunc: dmc.UpdateDaemonSet,
		},
	})
	// nolint:errcheck
	kdsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			ds, ok := obj.(*kosmosv1alpha1.DaemonSet)
			if !ok {
				return false
			}
			return ds.Annotations[MirrorAnnotation] == "true"
		},
		Handler: cache.ResourceEventHandlerFuncs{
			UpdateFunc: dmc.UpdateKosmosDaemonSet,
			DeleteFunc: dmc.DeleteKosmosDaemonSet,
		},
	})

	return dmc
}

func (dmc *DaemonSetsMirrorController) Run(ctx context.Context, workers int) {
	klog.Infof("starting daemon set mirror controller")
	defer klog.Infof("shutting down daemon set mirror controller")

	opt := utils.Options{
		Name: "distribute controller: KNode",
		KeyFunc: func(obj interface{}) (utils.QueueKey, error) {
			return keys.ClusterWideKeyFunc(obj)
		},
		ReconcileFunc:      dmc.syncDaemonSet,
		RateLimiterOptions: dmc.rateLimiterOptions,
	}
	dmc.processor = utils.NewAsyncWorker(opt)

	if !cache.WaitForNamedCacheSync("daemon set mirror controller", ctx.Done(),
		dmc.daemonSetSynced, dmc.kosmosDaemonSetSynced) {
		return
	}

	dmc.processor.Run(workers, ctx.Done())
	<-ctx.Done()
}

func (dmc *DaemonSetsMirrorController) syncDaemonSet(key utils.QueueKey) error {
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.V(2).Infof("invalid key type %T", key)
		return fmt.Errorf("invalid key")
	}

	namespace := clusterWideKey.Namespace
	name := clusterWideKey.Name

	d, err := dmc.dsLister.DaemonSets(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		klog.V(3).Infof("daemon set has been deleted %v", key)
		return nil
	}
	ds := d.DeepCopy()
	if ds.Annotations[MirrorAnnotation] != "true" {
		klog.V(4).Infof("daemon set %v is as not a mirror daemon set", key)
		return nil
	}

	if ds.DeletionTimestamp != nil {
		return nil
	}

	kd, err := dmc.kdsLister.DaemonSets(ds.Namespace).Get(ds.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ds := &kosmosv1alpha1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   ds.Namespace,
					Name:        ds.Name,
					Annotations: ds.Annotations,
					Labels:      ds.Labels,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "DaemonSet",
							Name:       ds.Name,
							UID:        ds.UID,
						},
					},
				},
				Spec: kosmosv1alpha1.DaemonSetSpec{
					Template:             ds.Spec.Template,
					Selector:             ds.Spec.Selector,
					UpdateStrategy:       ds.Spec.UpdateStrategy,
					MinReadySeconds:      ds.Spec.MinReadySeconds,
					RevisionHistoryLimit: ds.Spec.RevisionHistoryLimit,
				},
			}
			_, err := dmc.kosmosClient.KosmosV1alpha1().DaemonSets(ds.Namespace).Create(context.Background(), ds, metav1.CreateOptions{})
			if err != nil {
				klog.V(2).Infof("failed to create kosmos daemon set %v", err)
				return err
			}
			return nil
		} else {
			klog.V(2).Infof("failed to get kosmos daemon set %v", err)
			return err
		}
	}
	kds := kd.DeepCopy()
	if !isOwnedBy(kds.OwnerReferences, ds) {
		return nil
	}
	kds.Spec.Template = ds.Spec.Template
	kds.Spec.Selector = ds.Spec.Selector
	kds.Spec.UpdateStrategy = ds.Spec.UpdateStrategy
	kds.Spec.MinReadySeconds = ds.Spec.MinReadySeconds
	kds.Spec.RevisionHistoryLimit = ds.Spec.RevisionHistoryLimit
	kds.Labels = ds.Labels
	kds.Annotations = ds.Annotations
	kds.Labels = labels.Set{ManagedLabel: ""}
	kds, err = dmc.kosmosClient.KosmosV1alpha1().DaemonSets(ds.Namespace).Update(context.Background(), kds, metav1.UpdateOptions{})
	if err != nil {
		klog.V(2).Infof("failed to update shadow daemon set %v", err)
		return err
	}
	ds.Status.CurrentNumberScheduled = kds.Status.CurrentNumberScheduled
	ds.Status.NumberMisscheduled = kds.Status.NumberMisscheduled
	ds.Status.DesiredNumberScheduled = kds.Status.DesiredNumberScheduled
	ds.Status.NumberReady = kds.Status.NumberReady
	ds.Status.ObservedGeneration = kds.Status.ObservedGeneration
	ds.Status.UpdatedNumberScheduled = kds.Status.UpdatedNumberScheduled
	ds.Status.NumberAvailable = kds.Status.NumberAvailable
	ds.Status.NumberUnavailable = kds.Status.NumberUnavailable
	ds.Status.CollisionCount = kds.Status.CollisionCount
	ds.Status.Conditions = kds.Status.Conditions
	_, err = dmc.kubeClient.AppsV1().DaemonSets(ds.Namespace).UpdateStatus(context.Background(), ds, metav1.UpdateOptions{})
	if err != nil {
		klog.V(2).Infof("failed to update daemon set status %v", err)
		return err
	}
	return nil
}

func (dmc *DaemonSetsMirrorController) AddDaemonSet(obj interface{}) {
	ds, ok := obj.(*appsv1.DaemonSet)
	if !ok {
		return
	}
	dmc.processor.Enqueue(ds)
}

func (dmc *DaemonSetsMirrorController) UpdateDaemonSet(old interface{}, new interface{}) {
	ds, ok := new.(*appsv1.DaemonSet)
	if !ok {
		return
	}
	dmc.processor.Enqueue(ds)
}

func (dmc *DaemonSetsMirrorController) ProcessKosmosDaemonSet(obj interface{}) {
	kds, ok := obj.(*kosmosv1alpha1.DaemonSet)
	if !ok {
		return
	}
	ds, err := dmc.dsLister.DaemonSets(kds.Namespace).Get(kds.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.V(2).Infof("failed to get daemon set %v", err)
		}
		return
	}
	dmc.processor.Enqueue(ds)
}

func (dmc *DaemonSetsMirrorController) DeleteKosmosDaemonSet(obj interface{}) {
	dmc.ProcessKosmosDaemonSet(obj)
}

func (dmc *DaemonSetsMirrorController) UpdateKosmosDaemonSet(old interface{}, new interface{}) {
	dmc.ProcessKosmosDaemonSet(new)
}
