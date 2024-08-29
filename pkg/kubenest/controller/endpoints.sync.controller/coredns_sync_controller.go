package endpointcontroller

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

type CoreDNSController struct {
	client.Client
	EventRecorder record.EventRecorder
}

const CoreDNSSyncControllerName = "virtual-cluster-coredns-sync-controller"

func (e *CoreDNSController) SetupWithManager(mgr manager.Manager) error {
	skipEvent := func(obj client.Object) bool {
		// Only handle the "kube-dns" service with namespacing
		return obj.GetName() == constants.KubeDNSSVCName && obj.GetNamespace() != ""
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(CoreDNSSyncControllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(&v1.Service{},
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(createEvent event.CreateEvent) bool {
					return skipEvent(createEvent.Object)
				},
				UpdateFunc: func(updateEvent event.UpdateEvent) bool { return skipEvent(updateEvent.ObjectNew) },
				DeleteFunc: func(deleteEvent event.DeleteEvent) bool { return false },
			})).
		Complete(e)
}

func (e *CoreDNSController) SyncVirtualClusterSVC(ctx context.Context, k8sClient kubernetes.Interface, DNSPort int32, DNSTCPPort int32, MetricsPort int32) error {
	virtualClusterSVC, err := k8sClient.CoreV1().Services(constants.SystemNs).Get(ctx, constants.KubeDNSSVCName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get virtualcluster svc %s failed: %v", constants.KubeDNSSVCName, err)
	}

	if virtualClusterSVC.Spec.Ports == nil {
		return fmt.Errorf("svc %s ports is nil", constants.KubeDNSSVCName)
	}

	updateSVC := virtualClusterSVC.DeepCopy()

	for i, port := range virtualClusterSVC.Spec.Ports {
		if port.Name == "dns" {
			updateSVC.Spec.Ports[i].TargetPort = intstr.IntOrString{Type: intstr.Int, IntVal: DNSPort}
		}
		if port.Name == "dns-tcp" {
			updateSVC.Spec.Ports[i].TargetPort = intstr.IntOrString{Type: intstr.Int, IntVal: DNSTCPPort}
		}
		if port.Name == "metrics" {
			updateSVC.Spec.Ports[i].TargetPort = intstr.IntOrString{Type: intstr.Int, IntVal: MetricsPort}
		}
	}

	if _, err := k8sClient.CoreV1().Services(constants.SystemNs).Update(ctx, updateSVC, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func (e *CoreDNSController) SyncVirtualClusterEPS(ctx context.Context, k8sClient kubernetes.Interface, DNSPort int32, DNSTCPPort int32, MetricsPort int32) error {
	virtualEndPoints, err := k8sClient.CoreV1().Endpoints(constants.SystemNs).Get(ctx, constants.KubeDNSSVCName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get virtualcluster eps %s failed: %v", constants.KubeDNSSVCName, err)
	}

	if len(virtualEndPoints.Subsets) != 1 {
		return fmt.Errorf("eps %s Subsets length is not 1", constants.KubeDNSSVCName)
	}

	if virtualEndPoints.Subsets[0].Ports == nil {
		return fmt.Errorf("eps %s ports length is nil", constants.KubeDNSSVCName)
	}

	updateEPS := virtualEndPoints.DeepCopy()

	for i, port := range virtualEndPoints.Subsets[0].Ports {
		if port.Name == "dns" {
			updateEPS.Subsets[0].Ports[i].Port = DNSPort
		}
		if port.Name == "dns-tcp" {
			updateEPS.Subsets[0].Ports[i].Port = DNSTCPPort
		}
		if port.Name == "metrics" {
			updateEPS.Subsets[0].Ports[i].Port = MetricsPort
		}
	}

	if _, err := k8sClient.CoreV1().Endpoints(constants.SystemNs).Update(ctx, updateEPS, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func (e *CoreDNSController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s start to reconcile %s ============", CoreDNSSyncControllerName, request.NamespacedName)
	defer klog.V(4).Infof("============ %s finish to reconcile %s ============", CoreDNSSyncControllerName, request.NamespacedName)

	// Find the corresponding virtualcluster based on the namespace of SVC
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

	if targetVirtualCluster.Spec.KubeInKubeConfig != nil && targetVirtualCluster.Spec.KubeInKubeConfig.UseTenantDns {
		return reconcile.Result{}, nil
	}

	// Get the corresponding svc
	var kubesvc v1.Service
	if err := e.Get(ctx, request.NamespacedName, &kubesvc); err != nil {
		klog.V(4).Infof("get kubesvc %s failed: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}
	dnsPort := int32(0)
	dnsTCPPort := int32(0)
	metricsPort := int32(0)

	for _, port := range kubesvc.Spec.Ports {
		if port.Name == "dns" {
			dnsPort = port.NodePort
		}
		if port.Name == "dns-tcp" {
			dnsTCPPort = port.NodePort
		}
		if port.Name == "metrics" {
			metricsPort = port.NodePort
		}
	}

	k8sClient, err := util.GenerateKubeclient(&targetVirtualCluster)
	if err != nil {
		klog.Errorf("virtualcluster %s crd kubernetes client failed: %v", targetVirtualCluster.Name, err)
		return reconcile.Result{}, nil
	}

	// do sync
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return e.SyncVirtualClusterEPS(ctx, k8sClient, dnsPort, dnsTCPPort, metricsPort)
	}); err != nil {
		klog.Errorf("virtualcluster %s sync virtualcluster endpoints failed: %v", targetVirtualCluster.Name, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return e.SyncVirtualClusterSVC(ctx, k8sClient, dnsPort, dnsTCPPort, metricsPort)
	}); err != nil {
		klog.Errorf("virtualcluster %s sync virtualcluster svc failed: %v", targetVirtualCluster.Name, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	return reconcile.Result{}, nil
}
