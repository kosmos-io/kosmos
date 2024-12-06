package endpointcontroller

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type APIServerExternalSyncController struct {
	client.Client
	EventRecorder record.EventRecorder
}

const APIServerExternalSyncControllerName string = "api-server-external-service-sync-controller"

func (e *APIServerExternalSyncController) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(APIServerExternalSyncControllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		Watches(&source.Kind{Type: &v1.Pod{}}, handler.EnqueueRequestsFromMapFunc(e.newPodMapFunc())).
		Complete(e)
}

func (e *APIServerExternalSyncController) newPodMapFunc() handler.MapFunc {
	return func(a client.Object) []reconcile.Request {
		var requests []reconcile.Request
		pod := a.(*v1.Pod)

		// pod 的名称包含 "apiserver" 并且不包含 "kube-apiserver"
		if strings.Contains(pod.Name, "apiserver") && !strings.Contains(pod.Name, "kube-apiserver") {
			klog.V(4).Infof("api-server-external-sync-controller: Detected change in apiserver Pod: %s", pod.Name)

			// 根据 pod 名称推断 vcluster 名称
			parts := strings.SplitN(pod.Name, "-apiserver", 2)
			vclusterName := parts[0]
			klog.V(4).Infof("Derived vclusterName: %s from podName: %s", vclusterName, pod.Name)

			// 查找与该 Pod 关联的 VirtualCluster
			vcluster := &v1alpha1.VirtualCluster{}
			if err := e.Client.Get(context.Background(), types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      vclusterName,
			}, vcluster); err != nil {
				klog.Errorf("Failed to get VirtualCluster %s: %v", vclusterName, err)
				return nil
			}

			// 确保 VirtualCluster 状态为 Completed
			if vcluster.Status.Phase == v1alpha1.Completed {
				klog.V(4).Infof("VirtualCluster %s is completed, enqueueing for reconciliation", vclusterName)
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      vcluster.Name,
						Namespace: vcluster.Namespace,
					},
				})
			}
		}
		return requests
	}
}

func (e *APIServerExternalSyncController) SyncAPIServerExternalEPS(ctx context.Context, k8sClient kubernetes.Interface, vc *v1alpha1.VirtualCluster) error {
	podList := &v1.PodList{}
	if err := e.Client.List(ctx, podList, &client.ListOptions{
		Namespace: vc.Namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"virtualCluster-app": "apiserver",
		}),
	}); err != nil {
		return fmt.Errorf("failed to list apiserver pods: %w", err)
	}

	var addresses []v1.EndpointAddress
	for _, pod := range podList.Items {
		// 确保 Pod 处于 Running 状态并有 IP 地址
		if pod.Status.Phase == v1.PodRunning && pod.Status.PodIP != "" {
			klog.V(4).Infof("Found apiserver Pod: %s, IP: %s", pod.Name, pod.Status.PodIP)
			addresses = append(addresses, v1.EndpointAddress{IP: pod.Status.PodIP})
		}
	}

	apiServerPort, ok := vc.Status.PortMap[constants.APIServerPortKey]
	if !ok {
		return fmt.Errorf("failed to get API server port from VirtualCluster status")
	}
	klog.V(4).Infof("API server port: %d", apiServerPort)

	// constructing Endpoints objects
	newEndpoint := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.APIServerExternalService,
			Namespace: constants.KosmosNs,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: addresses,
				Ports: []v1.EndpointPort{
					{
						Name:     "https",
						Port:     apiServerPort,
						Protocol: v1.ProtocolTCP,
					},
				},
			},
		},
	}

	//avoid unnecessary updates
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentEndpoint, err := k8sClient.CoreV1().Endpoints(constants.KosmosNs).Get(ctx, constants.APIServerExternalService, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			_, err := k8sClient.CoreV1().Endpoints(constants.KosmosNs).Create(ctx, newEndpoint, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create api-server-external-service endpoint: %w", err)
			}
			klog.Infof("Created api-server-external-service Endpoint")
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to get existing api-server-external-service endpoint: %w", err)
		}

		// determine if an update is needed
		if !endpointsEqual(currentEndpoint, newEndpoint) {
			_, err := k8sClient.CoreV1().Endpoints(constants.KosmosNs).Update(ctx, newEndpoint, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update api-server-external-service endpoint: %w", err)
			}
			klog.Infof("Updated api-server-external-service Endpoint")
		} else {
			klog.V(4).Info("No changes detected in Endpoint, skipping update")
		}
		return nil
	})
}

// Endpoints 比较函数
func endpointsEqual(a, b *v1.Endpoints) bool {
	return fmt.Sprintf("%v", a.Subsets) == fmt.Sprintf("%v", b.Subsets)
}

func (e *APIServerExternalSyncController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s start to reconcile %s ============", APIServerExternalSyncControllerName, request.NamespacedName)
	defer klog.V(4).Infof("============ %s finish to reconcile %s ============", APIServerExternalSyncControllerName, request.NamespacedName)

	var vc v1alpha1.VirtualCluster
	if err := e.Get(ctx, request.NamespacedName, &vc); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("VirtualCluster not found: %s", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		klog.Errorf("Failed to get VirtualCluster: %v", err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	if vc.Status.Phase != v1alpha1.Completed {
		klog.Infof("VirtualCluster %s is not in Completed phase", vc.Name)
		return reconcile.Result{}, nil
	}

	k8sClient, err := util.GenerateKubeclient(&vc)
	if err != nil {
		klog.Errorf("Failed to generate Kubernetes client for VirtualCluster %s: %v", vc.Name, err)
		return reconcile.Result{}, nil
	}

	if err := e.SyncAPIServerExternalEPS(ctx, k8sClient, &vc); err != nil {
		klog.Errorf("Failed to sync apiserver external Endpoints: %v", err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	return reconcile.Result{}, nil
}
