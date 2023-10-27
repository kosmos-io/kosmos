package pod

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
)

const (
	LeafPodControllerName = "leaf-pod-controller"
	LeafPodRequeueTime    = 10 * time.Second
)

type LeafPodReconciler struct {
	client.Client
	RootClient client.Client
	Namespace  string
}

func (r *LeafPodReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	if request.NamespacedName.Namespace == utils.ReservedNS {
		return reconcile.Result{}, nil
	}

	// skip namespace
	if len(r.Namespace) > 0 && r.Namespace != request.NamespacedName.Namespace {
		return reconcile.Result{}, nil
	}

	var pod corev1.Pod
	if err := r.Get(ctx, request.NamespacedName, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			// delete pod in root
			if err := r.safeDeletePodInRootCluster(ctx, request); err != nil {
				return reconcile.Result{RequeueAfter: LeafPodRequeueTime}, nil
			}
			return reconcile.Result{}, nil
		}
		klog.Errorf("get %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: LeafPodRequeueTime}, nil
	}

	podCopy := pod.DeepCopy()

	if ShouldSkipStatusUpdate(podCopy) {
		return reconcile.Result{}, nil
	}

	if podutils.IsKosmosPod(podCopy) {
		podutils.FitObjectMeta(&podCopy.ObjectMeta)
		podCopy.ResourceVersion = "0"
		if err := r.RootClient.Status().Update(ctx, podCopy); err != nil && !apierrors.IsNotFound(err) {
			klog.Info(errors.Wrap(err, "error while updating pod status in kubernetes"))
			return reconcile.Result{RequeueAfter: LeafPodRequeueTime}, nil
		}
	}
	return reconcile.Result{}, nil
}

type rootDeleteOption struct {
	GracePeriodSeconds *int64
}

func (dopt *rootDeleteOption) ApplyToDelete(opt *client.DeleteOptions) {
	opt.GracePeriodSeconds = dopt.GracePeriodSeconds
}

func NewRootDeleteOption(pod *corev1.Pod) client.DeleteOption {
	gracePeriodSeconds := pod.DeletionGracePeriodSeconds

	current := metav1.NewTime(time.Now())
	if pod.DeletionTimestamp.Before(&current) {
		gracePeriodSeconds = new(int64)
	}

	return &rootDeleteOption{
		GracePeriodSeconds: gracePeriodSeconds,
	}
}

func (r *LeafPodReconciler) safeDeletePodInRootCluster(ctx context.Context, request reconcile.Request) error {
	rPod := corev1.Pod{}
	err := r.RootClient.Get(ctx, request.NamespacedName, &rPod)
	if err == nil || !apierrors.IsNotFound(err) {
		rPodCopy := rPod.DeepCopy()

		deleteOption := NewRootDeleteOption(rPodCopy)

		if err := r.RootClient.Delete(ctx, rPodCopy, deleteOption); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func (r *LeafPodReconciler) SetupWithManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(LeafPodControllerName).
		WithOptions(controller.Options{}).
		For(&corev1.Pod{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				pod1 := updateEvent.ObjectOld.(*corev1.Pod)
				pod2 := updateEvent.ObjectNew.(*corev1.Pod)
				return !cmp.Equal(pod1.Status, pod2.Status)
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return true
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return false
			},
		})).
		Complete(r)
}

func ShouldSkipStatusUpdate(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded ||
		pod.Status.Phase == corev1.PodFailed
}
