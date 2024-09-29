package pod

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
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
)

type LeafPodReconciler struct {
	client.Client
	RootClient client.Client
	Namespace  string
}

func (r *LeafPodReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var pod corev1.Pod
		if err := r.Get(ctx, request.NamespacedName, &pod); err != nil {
			if apierrors.IsNotFound(err) {
				// delete pod in root
				if err := DeletePodInRootCluster(ctx, request.NamespacedName, r.RootClient); err != nil {
					return err
				}
				return nil
			}
			klog.Errorf("get %s error: %v", request.NamespacedName, err)
			return err
		}

		podCopy := pod.DeepCopy()

		if podutils.IsKosmosPod(podCopy) {
			podutils.FitObjectMeta(&podCopy.ObjectMeta)
			if err := r.RootClient.Status().Update(ctx, podCopy); err != nil && !apierrors.IsNotFound(err) {
				klog.V(4).Info(errors.Wrap(err, "error while updating pod status in kubernetes"))
				return err
			}
		}
		return nil
	})
	if err != nil {
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, err
	}
	return reconcile.Result{}, nil
}

type rootDeleteOption struct {
	GracePeriodSeconds *int64
}

func (dopt *rootDeleteOption) ApplyToDelete(opt *client.DeleteOptions) {
	opt.GracePeriodSeconds = dopt.GracePeriodSeconds
}

func NewRootDeleteOption(_ *corev1.Pod) client.DeleteOption {
	// TODO
	//gracePeriodSeconds := pod.DeletionGracePeriodSeconds
	//
	//current := metav1.NewTime(time.Now())
	//if pod.DeletionTimestamp.Before(&current) {
	//	gracePeriodSeconds = new(int64)
	//}
	return &rootDeleteOption{
		GracePeriodSeconds: new(int64),
	}
}

func NewLeafDeleteOption(pod *corev1.Pod) client.DeleteOption {
	var gracePeriodSeconds *int64
	// Check if DeletionTimestamp is set and before the current time
	current := metav1.NewTime(time.Now())
	if pod.DeletionTimestamp != nil && pod.DeletionTimestamp.Before(&current) {
		//force
		gracePeriodSeconds = new(int64)
	} else {
		if pod.DeletionGracePeriodSeconds != nil {
			//DeletionGracePeriodSeconds determines how long it will take before the pod status changes to Termination
			//wait
			gracePeriodSeconds = pod.DeletionGracePeriodSeconds
		} else if pod.Spec.TerminationGracePeriodSeconds != nil {
			//TerminationGracePeriodSeconds is how long to wait after the pod is marked as Termination state before the container process is forcibly deleted.
			//wait
			gracePeriodSeconds = pod.Spec.TerminationGracePeriodSeconds
		}

		if pod.DeletionGracePeriodSeconds != nil && pod.Spec.TerminationGracePeriodSeconds != nil {
			//Sum both grace periods
			totalGracePeriod := *pod.DeletionGracePeriodSeconds + *pod.Spec.TerminationGracePeriodSeconds
			gracePeriodSeconds = &totalGracePeriod
		}
	}

	return &rootDeleteOption{
		GracePeriodSeconds: gracePeriodSeconds,
	}
}

func DeletePodInRootCluster(ctx context.Context, rootnamespacedname types.NamespacedName, rootClient client.Client) error {
	rPod := corev1.Pod{}
	err := rootClient.Get(ctx, rootnamespacedname, &rPod)

	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	rPodCopy := rPod.DeepCopy()
	deleteOption := NewRootDeleteOption(rPodCopy)

	if err := rootClient.Delete(ctx, rPodCopy, deleteOption); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (r *LeafPodReconciler) SetupWithManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	skipFunc := func(obj client.Object) bool {
		if obj.GetNamespace() == utils.ReservedNS {
			return false
		}

		// skip namespace
		if len(r.Namespace) > 0 && r.Namespace != obj.GetNamespace() {
			return false
		}

		p := obj.(*corev1.Pod)
		return podutils.IsKosmosPod(p)
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(LeafPodControllerName).
		WithOptions(controller.Options{}).
		For(&corev1.Pod{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				// ignore create event
				return skipFunc(createEvent.Object)
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				pod1 := updateEvent.ObjectOld.(*corev1.Pod)
				pod2 := updateEvent.ObjectNew.(*corev1.Pod)
				if !skipFunc(updateEvent.ObjectNew) {
					return false
				}
				return !cmp.Equal(pod1.Status, pod2.Status)
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return skipFunc(deleteEvent.Object)
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return false
			},
		})).
		Complete(r)
}
