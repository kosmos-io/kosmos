package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/adapters"
	k8sadapter "github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/adapters/k8s"
	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/utils/manager"
	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/utils/podutils"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/flags"
)

const (
	podStatusReasonAdapterFailed = "adapterFailed"
	podEventCreateFailed         = "adapterCreateFailed"
	podEventCreateSuccess        = "adapterCreateSuccess"
	podEventDeleteFailed         = "adapterDeleteFailed"
	podEventDeleteSuccess        = "adapterDeleteSuccess"
	podEventUpdateFailed         = "adapterUpdateFailed"
	podEventUpdateSuccess        = "adapterUpdateSuccess"

	notificationRetryPeriod = 150 * time.Millisecond
)

type PodConfig struct {
	PodClient corev1client.PodsGetter

	PodInformer corev1informers.PodInformer

	EventRecorder record.EventRecorder

	PodHandler adapters.PodHandler

	ConfigMapInformer corev1informers.ConfigMapInformer
	SecretInformer    corev1informers.SecretInformer
	ServiceInformer   corev1informers.ServiceInformer

	RateLimiterOpts flags.Options
}

type attendPod struct {
	sync.Mutex
	lastPodStatusReceivedFromAdapter *corev1.Pod
	lastPodUsed                      *corev1.Pod
	lastPodStatusUpdateSkipped       bool
}

type PodController struct {
	client          corev1client.PodsGetter
	podsInformer    corev1informers.PodInformer
	podsLister      corev1listers.PodLister
	podHandler      adapters.PodHandler
	resourceManager *manager.ResourceManager

	recorder record.EventRecorder
	// queue
	asyncPodFromKubeWorker          utils.AsyncWorker
	deletePodFromKubeWorker         utils.AsyncWorker
	asyncPodStatusFromAdapterWorker utils.AsyncWorker

	attendPods sync.Map

	ctx context.Context
}

func NewPodController(cfg PodConfig) (*PodController, error) {
	if cfg.PodClient == nil {
		return nil, fmt.Errorf("missing pod client")
	}
	if cfg.EventRecorder == nil {
		return nil, fmt.Errorf("missing event recorder")
	}
	if cfg.PodInformer == nil {
		return nil, fmt.Errorf("missing pod informer")
	}
	if cfg.ConfigMapInformer == nil {
		return nil, fmt.Errorf("missing config map informer")
	}
	if cfg.SecretInformer == nil {
		return nil, fmt.Errorf("missing secret informer")
	}
	if cfg.ServiceInformer == nil {
		return nil, fmt.Errorf("missing service informer")
	}
	if cfg.PodHandler == nil {
		return nil, fmt.Errorf("missing podHandler")
	}

	rm, err := manager.NewResourceManager(
		cfg.PodInformer.Lister(),
		cfg.SecretInformer.Lister(),
		cfg.ConfigMapInformer.Lister(),
		cfg.ServiceInformer.Lister())
	if err != nil {
		return nil, errors.Wrap(err, "could not create resource manager")
	}

	pc := &PodController{
		client:          cfg.PodClient,
		podsInformer:    cfg.PodInformer,
		podsLister:      cfg.PodInformer.Lister(),
		podHandler:      cfg.PodHandler,
		resourceManager: rm,
		recorder:        cfg.EventRecorder,
	}

	keyFunc := func(obj interface{}) (utils.QueueKey, error) {
		return obj, nil
	}

	pc.asyncPodFromKubeWorker = utils.NewAsyncWorker(utils.Options{
		Name:               "asyncPodFromKube",
		KeyFunc:            keyFunc,
		ReconcileFunc:      pc.asyncPodFromKube,
		RateLimiterOptions: cfg.RateLimiterOpts,
	})

	pc.deletePodFromKubeWorker = utils.NewAsyncWorker(utils.Options{
		Name:               "deletePodFromKube",
		KeyFunc:            keyFunc,
		ReconcileFunc:      pc.deletePodFromKube,
		RateLimiterOptions: cfg.RateLimiterOpts,
	})

	pc.asyncPodStatusFromAdapterWorker = utils.NewAsyncWorker(utils.Options{
		Name:               "asyncPodStatusFromAdapterWorker",
		KeyFunc:            keyFunc,
		ReconcileFunc:      pc.asyncPodStatusFromAdapter,
		RateLimiterOptions: cfg.RateLimiterOpts,
	})

	return pc, nil
}

func (pc *PodController) Run(ctx context.Context, podSyncWorkers int) (retErr error) {
	pc.ctx = ctx
	ctx, cancel := context.WithCancel(pc.ctx)
	defer cancel()

	pc.podHandler.Notify(ctx, func(pod *corev1.Pod) {
		pc.enqueuePodStatusUpdate(ctx, pod.DeepCopy())
	})

	var eventHandler cache.ResourceEventHandler = cache.ResourceEventHandlerFuncs{
		AddFunc: func(pod interface{}) {
			podInstance := pod.(*corev1.Pod)
			if podInstance.Namespace == k8sadapter.ReservedNS {
				return
			}

			if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
				klog.Error(err)
			} else {
				pc.attendPods.Store(key, &attendPod{})
				pc.asyncPodFromKubeWorker.Add(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)

			if newPod.Namespace == k8sadapter.ReservedNS {
				return
			}

			if key, err := cache.MetaNamespaceKeyFunc(newPod); err != nil {
				klog.Error(err)
			} else {
				obj, ok := pc.attendPods.Load(key)
				if !ok {
					panic("Pod not found in attend pods. This should never happen.")
				}

				aPod := obj.(*attendPod)
				aPod.Lock()
				if aPod.lastPodStatusReceivedFromAdapter != nil {
					tmpPod := &corev1.Pod{
						Status: aPod.lastPodStatusReceivedFromAdapter.Status,
						ObjectMeta: metav1.ObjectMeta{
							Annotations: aPod.lastPodStatusReceivedFromAdapter.Annotations,
							Labels:      aPod.lastPodStatusReceivedFromAdapter.Labels,
							Finalizers:  aPod.lastPodStatusReceivedFromAdapter.Finalizers,
						},
					}
					if aPod.lastPodStatusUpdateSkipped && podutils.IsChange(newPod, tmpPod) {
						pc.asyncPodFromKubeWorker.Add(key)
						// TODO: Reset this to avoid re-adding it continuously
						aPod.lastPodStatusUpdateSkipped = false
					}
				}
				aPod.Unlock()

				if podutils.ShouldEnqueue(oldPod, newPod) {
					pc.asyncPodFromKubeWorker.Add(key)
				}
			}
		},
		DeleteFunc: func(pod interface{}) {
			podInstance := pod.(*corev1.Pod)

			if podInstance.Namespace == k8sadapter.ReservedNS {
				return
			}
			if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
				klog.Error(err)
			} else {
				k8sPod, ok := pod.(*corev1.Pod)
				if !ok {
					return
				}
				pc.attendPods.Delete(key)
				pc.asyncPodFromKubeWorker.Add(key)

				key = fmt.Sprintf("%v/%v", key, k8sPod.UID)
				pc.deletePodFromKubeWorker.Forget(key)
			}
		},
	}

	if _, err := pc.podsInformer.Informer().AddEventHandler(eventHandler); err != nil {
		return err
	}

	pc.asyncPodFromKubeWorker.Run(1, ctx.Done())
	pc.deletePodFromKubeWorker.Run(1, ctx.Done())
	pc.asyncPodStatusFromAdapterWorker.Run(1, ctx.Done())

	<-ctx.Done()

	return nil
}

func (pc *PodController) asyncPodFromKube(key utils.QueueKey) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key.(string))
	if err != nil {
		klog.Warningf("invalid meta namespace key: %q, err: %s", key, err)
		return nil
	}

	pod, err := pc.podsLister.Pods(namespace).Get(name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			// TODO: miss event
			return err
		}

		pod, err = pc.podHandler.Get(pc.ctx, namespace, name)
		if err != nil && !apierrors.IsNotFound(err) {
			err = errors.Wrapf(err, "failed to get pod with key %q from adapter", key)
			return err
		}
		if apierrors.IsNotFound(err) || pod == nil {
			return nil
		}

		err = pc.podHandler.Delete(pc.ctx, pod)
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			err = errors.Wrapf(err, "failed to delete pod %q in the adapter", fmt.Sprintf("%s/%s", namespace, name))
		}
		return err
	}

	return pc.syncPodInAdapter(pc.ctx, pod, key.(string))
}

func (pc *PodController) handleAdapterError(ctx context.Context, origErr error, pod *corev1.Pod) {
	podPhase := corev1.PodPending
	if pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
		podPhase = corev1.PodFailed
	}

	pod.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
	pod.Status.Phase = podPhase
	pod.Status.Reason = podStatusReasonAdapterFailed
	pod.Status.Message = origErr.Error()

	_, err := pc.client.Pods(pod.Namespace).UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		klog.Warningf("Failed to update pod status, err:", err)
	} else {
		klog.Info("Updated k8s pod status")
	}
}

func (pc *PodController) createOrUpdatePod(ctx context.Context, pod *corev1.Pod) error {
	pod = pod.DeepCopy()
	if err := podutils.PopulateEnvironmentVariables(ctx, pod, pc.resourceManager, pc.recorder); err != nil {
		return err
	}

	podForAdapter := pod.DeepCopy()

	if podFromAdapter, _ := pc.podHandler.Get(ctx, pod.Namespace, pod.Name); podFromAdapter != nil {
		if !podutils.IsEqual(podFromAdapter, podForAdapter) {
			klog.Infof("Pod %s exists, updating pod in adapter", podFromAdapter.Name)
			if origErr := pc.podHandler.Update(ctx, podForAdapter); origErr != nil {
				pc.handleAdapterError(ctx, origErr, pod)
				pc.recorder.Event(pod, corev1.EventTypeWarning, podEventUpdateFailed, origErr.Error())

				return origErr
			}
			klog.Info("Updated pod in adapter")
			pc.recorder.Event(pod, corev1.EventTypeNormal, podEventUpdateSuccess, "Update pod in adapter successfully")
		}
	} else {
		if origErr := pc.podHandler.Create(ctx, podForAdapter); origErr != nil {
			pc.handleAdapterError(ctx, origErr, pod)
			pc.recorder.Event(pod, corev1.EventTypeWarning, podEventCreateFailed, origErr.Error())
			return origErr
		}
		klog.Info("Created pod in adapter")
		pc.recorder.Event(pod, corev1.EventTypeNormal, podEventCreateSuccess, "Create pod in adapter successfully")
	}
	return nil
}

func (pc *PodController) syncPodInAdapter(ctx context.Context, pod *corev1.Pod, key string) (retErr error) {
	klog.Info("processing pod sync")

	if pod.DeletionTimestamp != nil && !podutils.IsRunning(&pod.Status) {
		klog.Info("Force deleting pod from API Server as it is no longer running")
		pc.deletePodFromKubeWorker.Add(key)
		key = fmt.Sprintf("%v/%v", key, pod.UID)
		pc.deletePodFromKubeWorker.Add(key)
		return nil
	}
	obj, ok := pc.attendPods.Load(key)
	if !ok {
		return nil
	}
	aPod := obj.(*attendPod)
	aPod.Lock()
	if aPod.lastPodUsed != nil && podutils.EffectivelyEqual(aPod.lastPodUsed, pod) {
		aPod.Unlock()
		return nil
	}
	aPod.Unlock()

	defer func() {
		if retErr == nil {
			aPod.Lock()
			defer aPod.Unlock()
			aPod.lastPodUsed = pod
		}
	}()

	if pod.DeletionTimestamp != nil {
		klog.Info("Deleting pod in adapter")
		if err := pc.podHandler.Delete(ctx, pod.DeepCopy()); apierrors.IsNotFound(err) {
			klog.Info("Pod not found in adapter")
		} else if err != nil {
			err := errors.Wrapf(err, "failed to delete pod %q in the adapter", podutils.LoggableName(pod))
			return err
		}

		key = fmt.Sprintf("%v/%v", key, pod.UID)
		// TODO: EnqueueWithoutRateLimitWithDelay
		pc.deletePodFromKubeWorker.Add(key)
		return nil
	}

	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		klog.Warningf("skipping sync of pod %q in %q phase", podutils.LoggableName(pod), pod.Status.Phase)
		return nil
	}

	if err := pc.createOrUpdatePod(ctx, pod); err != nil {
		err := errors.Wrapf(err, "failed to sync pod %q in the adapter", podutils.LoggableName(pod))
		return err
	}
	return nil
}

func (pc *PodController) deletePodFromKube(key utils.QueueKey) error {
	klog.Info("processing pod delete")
	uid, metaKey := podutils.GetUIDAndMetaNamespaceKey(key.(string))
	namespace, name, err := cache.SplitMetaNamespaceKey(metaKey)

	if err != nil {
		klog.Warning(errors.Wrapf(err, "invalid resource key: %q", key))
		return nil
	}

	k8sPod, err := pc.podsLister.Pods(namespace).Get(name)

	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if string(k8sPod.UID) != uid {
		klog.Warningf("Not deleting pod because remote pod %s has different UID: %s", k8sPod.UID, uid)
		return nil
	}
	if podutils.IsRunning(&k8sPod.Status) {
		klog.Error("Force deleting pod in running state")
	}

	deleteOptions := metav1.NewDeleteOptions(0)
	deleteOptions.Preconditions = metav1.NewUIDPreconditions(uid)
	err = pc.client.Pods(namespace).Delete(pc.ctx, name, *deleteOptions)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("Not deleting pod because %v", err)
			return nil
		}
		if apierrors.IsConflict(err) {
			klog.Warningf("There was a conflict, maybe trying to delete a Pod that has been recreated: %v", err)
			return nil
		}
		return err
	}

	return nil
}

func (pc *PodController) asyncPodStatusFromAdapter(key utils.QueueKey) (retErr error) {
	klog.Info("processing pod status update")
	defer func() {
		if retErr != nil {
			klog.Errorf("Error processing pod status update, err: %s", retErr)
		}
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key.(string))
	if err != nil {
		return errors.Wrap(err, "error splitting cache key")
	}

	pod, err := pc.podsLister.Pods(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Errorf("Skipping pod status update for pod missing in Kubernetes, err: %s", err)
			return nil
		}
		return errors.Wrap(err, "error looking up pod")
	}

	return pc.updatePodStatus(pc.ctx, pod, key)
}

func (pc *PodController) updatePodStatus(ctx context.Context, podFromKubernetes *corev1.Pod, key utils.QueueKey) error {
	if podutils.ShouldSkipStatusUpdate(podFromKubernetes) {
		return nil
	}

	obj, ok := pc.attendPods.Load(key)
	if !ok {
		// The pod has been deleted from K8s by other gorouting
		return nil
	}
	aPod := obj.(*attendPod)
	aPod.Lock()
	podFromAdapter := aPod.lastPodStatusReceivedFromAdapter.DeepCopy()
	aPod.Unlock()
	// Pod deleted by adapter.
	if podFromAdapter.DeletionTimestamp != nil && podFromKubernetes.DeletionTimestamp == nil {
		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: podFromAdapter.DeletionGracePeriodSeconds,
		}
		current := metav1.NewTime(time.Now())
		if podFromAdapter.DeletionTimestamp.Before(&current) {
			deleteOptions.GracePeriodSeconds = new(int64)
		}
		// check status here to avoid pod re-created deleted incorrectly.
		if cmp.Equal(podFromKubernetes.Status, podFromAdapter.Status) {
			if err := pc.client.Pods(podFromKubernetes.Namespace).Delete(ctx, podFromKubernetes.Name, deleteOptions); err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrap(err, "error while delete pod in kubernetes")
			}
		}
	}

	// copy the pod and set ResourceVersion to 0.
	podFromAdapter.ResourceVersion = "0"
	if _, err := pc.client.Pods(podFromKubernetes.Namespace).UpdateStatus(ctx, podFromAdapter, metav1.UpdateOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "error while updating pod status in kubernetes")
	}

	klog.Warningf("Updated pod status in kubernetes, new phase: %s, new reason: %s, old phase: %s, old reason: %s", string(podFromAdapter.Status.Phase), podFromAdapter.Status.Reason, string(podFromKubernetes.Status.Phase), podFromKubernetes.Status.Reason)

	return nil
}

func (pc *PodController) enqueuePodStatusUpdate(ctx context.Context, pod *corev1.Pod) {
	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		klog.Errorf("Error getting pod meta namespace key, err: %s", err)
		return
	}

	var obj interface{}
	err = wait.PollImmediateUntil(notificationRetryPeriod, func() (bool, error) {
		var ok bool
		obj, ok = pc.attendPods.Load(key)
		if ok {
			return true, nil
		}

		if !cache.WaitForNamedCacheSync("enqueuePodStatusUpdate", ctx.Done(), pc.podsInformer.Informer().HasSynced) {
			klog.Warning("enqueuePodStatusUpdate proceeding with unsynced cache")
		}

		_, err = pc.podsLister.Pods(pod.Namespace).Get(pod.Name)
		if err != nil {
			return false, err
		}

		return false, nil
	}, ctx.Done())

	if err != nil {
		if apierrors.IsNotFound(err) {
			err = fmt.Errorf("pod %q not found in pod lister: %v", key, err)
			klog.Errorf("Not enqueuing pod status update, err: %s", err)
		} else {
			klog.Warningf("Not enqueuing pod status update due to error from pod lister, err: %s", err)
		}
		return
	}

	apod := obj.(*attendPod)
	apod.Lock()
	if cmp.Equal(apod.lastPodStatusReceivedFromAdapter, pod) {
		apod.lastPodStatusUpdateSkipped = true
		apod.Unlock()
		return
	}
	apod.lastPodStatusUpdateSkipped = false
	apod.lastPodStatusReceivedFromAdapter = pod
	apod.Unlock()
	pc.asyncPodStatusFromAdapterWorker.Add(key)
}
