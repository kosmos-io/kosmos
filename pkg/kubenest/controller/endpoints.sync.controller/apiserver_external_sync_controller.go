package endpointcontroller

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
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

type ApiServerExternalSyncController struct {
	client.Client
	EventRecorder record.EventRecorder
}

const ApiServerExternalSyncControllerName string = "api-server-external-service-sync-controller"

func (e *ApiServerExternalSyncController) SetupWithManager(mgr manager.Manager) error {
	skipEvent := func(obj client.Object) bool {
		return strings.Contains(obj.GetName(), "apiserver") && obj.GetNamespace() != ""
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ApiServerExternalSyncControllerName).
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

func (e *ApiServerExternalSyncController) SyncApiServerExternalEPS(ctx context.Context, k8sClient kubernetes.Interface) error {
	kubeEndpoints, err := k8sClient.CoreV1().Endpoints(constants.DefaultNs).Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Error getting endpoints: %v", err)
		return fmt.Errorf("failed to get endpoints for kubernetes service: %v", err)
	} else {
		klog.Infof("Endpoints for service 'kubernetes': %v", kubeEndpoints)
		for _, subset := range kubeEndpoints.Subsets {
			for _, address := range subset.Addresses {
				klog.Infof("IP: %s", address.IP)
			}
		}
	}

	if len(kubeEndpoints.Subsets) != 1 {
		return fmt.Errorf("eps %s Subsets length is not 1", "kubernetes")
	}

	if kubeEndpoints.Subsets[0].Addresses == nil || len(kubeEndpoints.Subsets[0].Addresses) == 0 {
		return fmt.Errorf("eps %s Addresses length is nil", "kubernetes")
	}

	apiServerExternalEndpoints, err := k8sClient.CoreV1().Endpoints(constants.DefaultNs).Get(ctx, constants.ApiServerExternalService, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get endpoints for %s : %v", constants.ApiServerExternalService, err)
	}

	updateEPS := apiServerExternalEndpoints.DeepCopy()

	if apiServerExternalEndpoints != nil {
		klog.Infof("apiServerExternalEndpoints: %v", apiServerExternalEndpoints)
	} else {
		klog.Info("apiServerExternalEndpoints is nil")
	}

	if updateEPS != nil {
		klog.Infof("updateEPS: %v", updateEPS)
	} else {
		klog.Info("updateEPS is nil")
	}

	if len(updateEPS.Subsets) == 1 && len(updateEPS.Subsets[0].Addresses) == 1 {
		ip := kubeEndpoints.Subsets[0].Addresses[0].IP
		klog.Infof("IP address: %s", ip)
		updateEPS.Subsets[0].Addresses[0].IP = ip

		if _, err := k8sClient.CoreV1().Endpoints(constants.DefaultNs).Update(ctx, updateEPS, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update endpoints for api-server-external-service: %v", err)
		}
	} else {
		klog.ErrorS(err, "Unexpected format of endpoints for api-server-external-service", "endpoint_data", updateEPS)
		return fmt.Errorf("unexpected format of endpoints for api-server-external-service")
	}

	return nil
}

func (e *ApiServerExternalSyncController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s start to reconcile %s ============", ApiServerExternalSyncControllerName, request.NamespacedName)
	defer klog.V(4).Infof("============ %s finish to reconcile %s ============", ApiServerExternalSyncControllerName, request.NamespacedName)

	var virtualClusterList v1alpha1.VirtualClusterList
	if err := e.List(ctx, &virtualClusterList); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		klog.V(4).Infof("query virtualcluster failed: %v", err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}
	var targetVirtualCluster v1alpha1.VirtualCluster
	hasVirtualCluster := false
	for _, vc := range virtualClusterList.Items {
		if vc.Namespace == request.Namespace {
			targetVirtualCluster = vc
			klog.V(4).Infof("virtualcluster %s found", targetVirtualCluster.Name)
			hasVirtualCluster = true
			break
		}
	}
	if !hasVirtualCluster {
		klog.V(4).Infof("virtualcluster %s not found", request.Namespace)
		return reconcile.Result{}, nil
	}

	if targetVirtualCluster.Status.Phase != v1alpha1.AllNodeReady && targetVirtualCluster.Status.Phase != v1alpha1.Completed {
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	k8sClient, err := util.GenerateKubeclient(&targetVirtualCluster)
	if err != nil {
		klog.Errorf("virtualcluster %s crd kubernetes client failed: %v", targetVirtualCluster.Name, err)
		return reconcile.Result{}, nil
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return e.SyncApiServerExternalEPS(ctx, k8sClient)
	}); err != nil {
		klog.Errorf("virtualcluster %s sync apiserver external endpoints failed: %v", targetVirtualCluster.Name, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	return reconcile.Result{}, nil
}
