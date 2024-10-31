package endpointcontroller

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type KonnectivityController struct {
	client.Client
	EventRecorder record.EventRecorder
}

const KonnectivitySyncControllerName = "virtual-cluster-konnectivity-sync-controller"

func (e *KonnectivityController) SetupWithManager(mgr manager.Manager) error {
	skipEvent := func(obj client.Object) bool {
		// Only handle the "konnectivity-server" endpoints
		return strings.HasSuffix(obj.GetName(), constants.KonnectivityServerSuffix)
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(KonnectivitySyncControllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(&v1.Endpoints{},
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(createEvent event.CreateEvent) bool {
					return skipEvent(createEvent.Object)
				},
				UpdateFunc: func(updateEvent event.UpdateEvent) bool { return skipEvent(updateEvent.ObjectNew) },
				DeleteFunc: func(deleteEvent event.DeleteEvent) bool { return false },
			})).
		Complete(e)
}

func (e *KonnectivityController) SyncVirtualClusterEPS(ctx context.Context, k8sClient kubernetes.Interface, eps v1.Endpoints) error {
	virtualEndPoints, err := k8sClient.CoreV1().Endpoints(constants.SystemNs).Get(ctx, constants.KonnectivityServerSuffix, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get virtualcluster eps %s failed: %v", constants.KonnectivityServerSuffix, err)
	}

	if len(virtualEndPoints.Subsets) == 0 {
		return fmt.Errorf("virtualcluster eps %s has no subsets", constants.KonnectivityServerSuffix)
	}

	if len(virtualEndPoints.Subsets[0].Ports) == 0 {
		return fmt.Errorf("virtualcluster eps %s has no ports", constants.KonnectivityServerSuffix)
	}

	// fix bug: https://github.com/kosmos-io/kosmos/issues/683
	if len(eps.Subsets) == 0 {
		return fmt.Errorf("eps %s has no subsets", eps.Name)
	}

	// only sync the address of the konnectivity-server endpoints
	targetPort := virtualEndPoints.Subsets[0].Ports[0].Port
	updateEPS := virtualEndPoints.DeepCopy()

	copyFromEPS := eps.DeepCopy()
	updateEPS.Subsets = copyFromEPS.Subsets
	for i := range updateEPS.Subsets {
		if len(updateEPS.Subsets[i].Ports) == 0 {
			continue
		}
		updateEPS.Subsets[i].Ports[0].Port = targetPort
	}

	if _, err := k8sClient.CoreV1().Endpoints(constants.SystemNs).Update(ctx, updateEPS, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func (e *KonnectivityController) GetVirtualCluster(ctx context.Context, eps v1.Endpoints) (*v1alpha1.VirtualCluster, error) {
	virtualClusterName := strings.TrimSuffix(eps.GetName(), "-"+constants.KonnectivityServerSuffix)
	vartialClusterNamespace := eps.GetNamespace()
	var vc v1alpha1.VirtualCluster
	if err := e.Get(ctx, types.NamespacedName{Name: virtualClusterName, Namespace: vartialClusterNamespace}, &vc); err != nil {
		return nil, err
	}
	return &vc, nil
}

func (e *KonnectivityController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s start to reconcile %s ============", KonnectivitySyncControllerName, request.NamespacedName)
	defer klog.V(4).Infof("============ %s finish to reconcile %s ============", KonnectivitySyncControllerName, request.NamespacedName)

	// Get the corresponding svc
	var kubeEPS v1.Endpoints
	if err := e.Get(ctx, request.NamespacedName, &kubeEPS); err != nil {
		klog.V(4).Infof("get kubeEPS %s failed: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	targetVirtualCluster, err := e.GetVirtualCluster(ctx, kubeEPS)
	if err != nil {
		klog.V(4).Infof("query virtualcluster failed: %v", err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	if targetVirtualCluster.Status.Phase != v1alpha1.AllNodeReady && targetVirtualCluster.Status.Phase != v1alpha1.Completed {
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	if targetVirtualCluster.Spec.KubeInKubeConfig != nil && targetVirtualCluster.Spec.KubeInKubeConfig.APIServerServiceType == v1alpha1.NodePort {
		return reconcile.Result{}, nil
	}

	k8sClient, err := util.GenerateKubeclient(targetVirtualCluster)
	if err != nil {
		klog.Errorf("virtualcluster %s crd kubernetes client failed: %v", targetVirtualCluster.Name, err)
		return reconcile.Result{}, nil
	}

	// // do sync
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return e.SyncVirtualClusterEPS(ctx, k8sClient, kubeEPS)
	}); err != nil {
		klog.Errorf("virtualcluster %s sync virtualcluster svc failed: %v", targetVirtualCluster.Name, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	return reconcile.Result{}, nil
}
