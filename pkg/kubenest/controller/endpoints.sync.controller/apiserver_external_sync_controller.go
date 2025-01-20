package endpointcontroller

import (
	"context"
	"fmt"
	"reflect"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type NodeGetter interface {
	GetAPIServerNodes(client kubernetes.Interface, namespace string) (*v1.NodeList, error)
}

type RealNodeGetter struct{}

func (r *RealNodeGetter) GetAPIServerNodes(client kubernetes.Interface, namespace string) (*v1.NodeList, error) {
	return util.GetAPIServerNodes(client, namespace)
}

type APIServerExternalSyncController struct {
	client.Client
	EventRecorder record.EventRecorder
	KubeClient    kubernetes.Interface
	NodeGetter    NodeGetter
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
	return func(obj client.Object) []reconcile.Request {
		pod, ok := obj.(*v1.Pod)

		if !ok {
			klog.Warningf("Object is not a Pod, skipping: %v", obj)
			return nil
		}

		// If the pod contains the specified label virtualCluster-app=apiserver，it indicates that it belongs to vc.
		if val, exists := pod.Labels[constants.Label]; exists && val == constants.LabelValue {
			return []reconcile.Request{
				{
					NamespacedName: client.ObjectKey{
						Name:      pod.Name,
						Namespace: pod.Namespace,
					},
				},
			}
		}

		return nil
	}
}

func (e *APIServerExternalSyncController) SyncAPIServerExternalEndpoints(ctx context.Context, k8sClient kubernetes.Interface, vc *v1alpha1.VirtualCluster) error {
	if e.NodeGetter == nil {
		return fmt.Errorf("NodeGetter is nil")
	}

	nodes, err := e.NodeGetter.GetAPIServerNodes(e.KubeClient, vc.Namespace)
	if err != nil {
		return fmt.Errorf("failed to get API server nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return fmt.Errorf("no API server nodes found in the cluster")
	}

	var addresses []v1.EndpointAddress
	for _, node := range nodes.Items {
		for _, address := range node.Status.Addresses {
			if address.Type == v1.NodeInternalIP {
				addresses = append(addresses, v1.EndpointAddress{
					IP: address.Address,
				})
			}
		}
	}

	if len(addresses) == 0 {
		return fmt.Errorf("no internal IP addresses found for the API server nodes")
	}

	apiServerPort, ok := vc.Status.PortMap[constants.APIServerPortKey]
	if !ok {
		return fmt.Errorf("failed to get API server port from VirtualCluster status")
	}
	klog.V(4).Infof("API server port: %d", apiServerPort)

	newEndpoint := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.APIServerExternalService,
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

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := k8sClient.CoreV1().Namespaces().Get(ctx, constants.KosmosNs, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to get namespace kosmos-system: %w", err)
			}

			currentEndpoint, err := k8sClient.CoreV1().Endpoints(constants.DefaultNs).Get(ctx, constants.APIServerExternalService, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.V(4).Info("No endpoint found in default namespace, skipping")
					return nil
				}
				return fmt.Errorf("failed to get endpoint in default: %w", err)
			}

			if !reflect.DeepEqual(currentEndpoint.Subsets, newEndpoint.Subsets) {
				newEndpoint.ObjectMeta.Namespace = constants.DefaultNs
				newEndpoint.ObjectMeta.ResourceVersion = currentEndpoint.ResourceVersion
				_, err = k8sClient.CoreV1().Endpoints(constants.DefaultNs).Update(ctx, newEndpoint, metav1.UpdateOptions{})
				if err != nil {
					return fmt.Errorf("failed to update endpoint in default: %w", err)
				}
				klog.V(4).Info("Updated api-server-external-service Endpoint in default")
			} else {
				klog.V(4).Info("No changes detected in default Endpoint, skipping update")
			}
			return nil
		}

		currentEndpoint, err := k8sClient.CoreV1().Endpoints(constants.KosmosNs).Get(ctx, constants.APIServerExternalService, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				newEndpoint.ObjectMeta.Namespace = constants.KosmosNs
				_, err = k8sClient.CoreV1().Endpoints(constants.KosmosNs).Create(ctx, newEndpoint, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create endpoint in kosmos-system: %w", err)
				}
				klog.V(4).Info("Created api-server-external-service Endpoint in kosmos-system")
				return nil
			}
			return fmt.Errorf("failed to get endpoint in kosmos-system: %w", err)
		}

		if !reflect.DeepEqual(currentEndpoint.Subsets, newEndpoint.Subsets) {
			newEndpoint.ObjectMeta.Namespace = constants.KosmosNs
			newEndpoint.ObjectMeta.ResourceVersion = currentEndpoint.ResourceVersion
			_, err = k8sClient.CoreV1().Endpoints(constants.KosmosNs).Update(ctx, newEndpoint, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update endpoint in kosmos-system: %w", err)
			}
			klog.V(4).Info("Updated api-server-external-service Endpoint in kosmos-system")
		} else {
			klog.V(4).Info("No changes detected in kosmos-system Endpoint, skipping update")
		}
		return nil
	})
}

func (e *APIServerExternalSyncController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s start to reconcile %s ============", APIServerExternalSyncControllerName, request.NamespacedName)
	defer klog.V(4).Infof("============ %s finish to reconcile %s ============", APIServerExternalSyncControllerName, request.NamespacedName)

	var vcList v1alpha1.VirtualClusterList
	if err := e.List(ctx, &vcList, client.InNamespace(request.NamespacedName.Namespace)); err != nil {
		klog.Errorf("Failed to list VirtualClusters in namespace %s: %v", request.NamespacedName.Namespace, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	if len(vcList.Items) == 0 {
		klog.V(4).Infof("No VirtualCluster found in namespace %s", request.NamespacedName.Namespace)
		return reconcile.Result{}, nil
	}

	// A namespace should correspond to only one virtual cluster (vc). If it corresponds to multiple vcs, it indicates an error.
	if len(vcList.Items) > 1 {
		klog.Errorf("Multiple VirtualClusters found in namespace %s, expected only one", request.NamespacedName.Namespace)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	vc := vcList.Items[0]

	if vc.Status.Phase != v1alpha1.Completed {
		klog.V(4).Infof("VirtualCluster %s is not in Completed phase", vc.Name)
		return reconcile.Result{}, nil
	}

	k8sClient, err := util.GenerateKubeclient(&vc)
	if err != nil {
		klog.Errorf("Failed to generate Kubernetes client for VirtualCluster %s: %v", vc.Name, err)
		return reconcile.Result{}, nil
	}

	if err := e.SyncAPIServerExternalEndpoints(ctx, k8sClient, &vc); err != nil {
		klog.Errorf("Failed to sync apiserver external Endpoints: %v", err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	return reconcile.Result{}, nil
}
