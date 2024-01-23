package pv

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
)

const (
	RootPVControllerName = "root-pv-controller"
)

type RootPVController struct {
	RootClient        client.Client
	GlobalLeafManager leafUtils.LeafResourceManager
}

func (r *RootPVController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	pv := &v1.PersistentVolume{}
	shouldDelete := false
	err := r.RootClient.Get(ctx, request.NamespacedName, pv)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Warningf("get pv from root cluster failed, error: %v", err)
			return reconcile.Result{RequeueAfter: requeueTime}, nil
		}
		shouldDelete = true
		pv.Namespace = request.Namespace
		pv.Name = request.Name
	}

	if !pv.DeletionTimestamp.IsZero() || shouldDelete {
		return r.cleanupPv(pv)
	}
	return reconcile.Result{}, nil
}

func (r *RootPVController) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(RootPVControllerName).
		WithOptions(controller.Options{}).
		For(&v1.PersistentVolume{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				return false
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				if deleteEvent.DeleteStateUnknown {
					//TODO ListAndDelete
					klog.Warningf("missing delete pv root event %q", deleteEvent.Object.GetName())
					return false
				}

				pv := deleteEvent.Object.(*v1.PersistentVolume)
				// skip  one way pv, oneway_pv_controller will handle this PV
				if podutils.IsOneWayPV(pv) {
					return false
				}

				clusters := utils.ListResourceClusters(pv.Annotations)
				if len(clusters) == 0 {
					klog.Warningf("pv leaf %q doesn't existed", deleteEvent.Object.GetName())
					return false
				}

				lr, err := r.GlobalLeafManager.GetLeafResource(clusters[0])
				if err != nil {
					klog.Warningf("pv leaf %q doesn't existed in LeafResources", deleteEvent.Object.GetName())
					return false
				}

				if err = lr.Clientset.CoreV1().PersistentVolumes().Delete(context.TODO(), deleteEvent.Object.GetName(),
					metav1.DeleteOptions{}); err != nil {
					if !errors.IsNotFound(err) {
						klog.Errorf("delete pv from leaf cluster failed, %q, error: %v", deleteEvent.Object.GetName(), err)
					}
				}

				return false
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return false
			},
		})).
		Complete(r)
}

func (r *RootPVController) cleanupPv(pv *v1.PersistentVolume) (reconcile.Result, error) {
	clusters := utils.ListResourceClusters(pv.Annotations)
	if len(clusters) == 0 {
		klog.Warningf("pv leaf %q doesn't existed", pv.GetName())
		return reconcile.Result{}, nil
	}

	lr, err := r.GlobalLeafManager.GetLeafResource(clusters[0])
	if err != nil {
		klog.Warningf("pv leaf %q doesn't existed in LeafResources", pv.GetName())
		return reconcile.Result{}, nil
	}

	if err = lr.Clientset.CoreV1().PersistentVolumes().Delete(context.TODO(), pv.GetName(),
		metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			klog.Errorf("delete pv from leaf cluster failed, %q, error: %v", pv.GetName(), err)
			return reconcile.Result{RequeueAfter: requeueTime}, nil
		}
	}
	return reconcile.Result{}, nil
}
