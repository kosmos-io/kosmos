package mcs

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/strings/slices"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	clustertreeutils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const AutoCreateMCSControllerName = "auto-mcs-controller"

// AutoCreateMCSController watches services in root cluster and auto create serviceExport and serviceImport in leaf cluster
type AutoCreateMCSController struct {
	RootClient        client.Client
	RootKosmosClient  kosmosversioned.Interface
	EventRecorder     record.EventRecorder
	Logger            logr.Logger
	GlobalLeafManager clustertreeutils.LeafResourceManager
	// AutoCreateMCSPrefix are the prefix of the namespace for service to auto create in leaf cluster
	AutoCreateMCSPrefix []string
	// ReservedNamespaces are the protected namespaces to prevent Kosmos for deleting system resources
	ReservedNamespaces []string
}

func (c *AutoCreateMCSController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", AutoCreateMCSControllerName, request.NamespacedName.String())
	defer func() {
		klog.V(4).Infof("============ %s has been reconciled =============", request.NamespacedName.String())
	}()

	var shouldDelete bool
	service := &corev1.Service{}
	if err := c.RootClient.Get(ctx, request.NamespacedName, service); err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Cloud not get service in root cluster,Error: %v", err)
			return controllerruntime.Result{Requeue: true}, err
		}
		shouldDelete = true
	}

	if !matchNamespace(service.Namespace, c.AutoCreateMCSPrefix) && !hasAutoMCSAnnotation(service) {
		shouldDelete = true
	}

	clusterList := &kosmosv1alpha1.ClusterList{}
	if err := c.RootClient.List(ctx, clusterList); err != nil {
		klog.Errorf("Cloud not get cluster in root cluster,Error: %v", err)
		return controllerruntime.Result{Requeue: true}, err
	}

	// The service is being deleted, in which case we should clear serviceExport and serviceImport.
	if shouldDelete || !service.DeletionTimestamp.IsZero() {
		if err := c.cleanUpMcsResources(ctx, request.Namespace, request.Name, clusterList); err != nil {
			return controllerruntime.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
		}
		return controllerruntime.Result{}, nil
	}

	err := c.autoCreateMcsResources(ctx, service, clusterList)
	if err != nil {
		return controllerruntime.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}
	return controllerruntime.Result{}, nil
}

func matchNamespace(namespace string, prefix []string) bool {
	for _, p := range prefix {
		if strings.HasPrefix(namespace, p) {
			return true
		}
	}
	return false
}

func hasAutoMCSAnnotation(service *corev1.Service) bool {
	annotations := service.GetAnnotations()
	if annotations == nil {
		return false
	}
	if _, exists := annotations[utils.AutoCreateMCSAnnotation]; exists {
		return true
	}
	return false
}

func (c *AutoCreateMCSController) shouldEnqueue(service *corev1.Service) bool {
	if slices.Contains(c.ReservedNamespaces, service.Namespace) {
		return false
	}

	if len(c.AutoCreateMCSPrefix) > 0 {
		for _, prefix := range c.AutoCreateMCSPrefix {
			if strings.HasPrefix(service.GetNamespace(), prefix) {
				return true
			}
		}
	}

	if hasAutoMCSAnnotation(service) {
		return true
	}
	return false
}

func (c *AutoCreateMCSController) SetupWithManager(mgr manager.Manager) error {
	clusterFn := handler.MapFunc(
		func(object client.Object) []reconcile.Request {
			requestList := make([]reconcile.Request, 0)
			serviceList := &corev1.ServiceList{}
			err := c.RootClient.List(context.TODO(), serviceList)
			if err != nil {
				klog.Errorf("Can not get service in root cluster,Error: %v", err)
				return nil
			}
			for _, service := range serviceList.Items {
				request := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: service.Namespace,
						Name:      service.Name,
					},
				}
				requestList = append(requestList, request)
			}
			return requestList
		},
	)

	clusterPredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return false
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	},
	)

	servicePredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			service, ok := event.Object.(*corev1.Service)
			if !ok {
				return false
			}

			return c.shouldEnqueue(service)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			service, ok := deleteEvent.Object.(*corev1.Service)
			if !ok {
				return false
			}

			return c.shouldEnqueue(service)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			newService, ok := updateEvent.ObjectNew.(*corev1.Service)
			if !ok {
				return false
			}

			oldService, ok := updateEvent.ObjectOld.(*corev1.Service)
			if !ok {
				return false
			}

			return c.shouldEnqueue(newService) != c.shouldEnqueue(oldService)
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	},
	)

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&corev1.Service{}, servicePredicate).
		Watches(&source.Kind{Type: &kosmosv1alpha1.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(clusterFn),
			clusterPredicate,
		).
		Complete(c)
}

func (c *AutoCreateMCSController) cleanUpMcsResources(ctx context.Context, namespace string, name string, clusterList *kosmosv1alpha1.ClusterList) error {
	// delete serviceExport in root cluster
	if err := c.RootKosmosClient.MulticlusterV1alpha1().ServiceExports(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Delete serviceExport in root cluster failed %s/%s, Error: %v", namespace, name, err)
			return err
		}
	}
	// delete serviceImport in all leaf cluster
	for _, cluster := range clusterList.Items {
		newCluster := cluster.DeepCopy()
		if clustertreeutils.IsRootCluster(newCluster) {
			continue
		}

		clusterName := clustertreeutils.GetLeafResourceClusterName(newCluster)
		leafManager, err := c.GlobalLeafManager.GetLeafResource(clusterName)
		if err != nil {
			klog.Errorf("get leafManager for cluster %s failed,Error: %v", clusterName, err)
			return err
		}
		if err = leafManager.KosmosClient.MulticlusterV1alpha1().ServiceImports(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				klog.Errorf("Delete serviceImport in leaf cluster failed %s/%s, Error: %v", namespace, name, err)
				return err
			}
		}
	}
	return nil
}

func (c *AutoCreateMCSController) autoCreateMcsResources(ctx context.Context, service *corev1.Service, clusterList *kosmosv1alpha1.ClusterList) error {
	// create serviceExport in root cluster
	serviceExport := &mcsv1alpha1.ServiceExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: service.Namespace,
		},
	}
	if _, err := c.RootKosmosClient.MulticlusterV1alpha1().ServiceExports(service.Namespace).Create(ctx, serviceExport, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			klog.Errorf("Could not create serviceExport(%s/%s) in root cluster, Error: %v", service.Namespace, service.Name, err)
			return err
		}
	}

	// create serviceImport in leaf cluster
	for _, cluster := range clusterList.Items {
		newCluster := cluster.DeepCopy()
		if clustertreeutils.IsRootCluster(newCluster) {
			continue
		}

		clusterName := clustertreeutils.GetLeafResourceClusterName(newCluster)
		leafManager, err := c.GlobalLeafManager.GetLeafResource(clusterName)
		if err != nil {
			klog.Errorf("get leafManager for cluster %s failed,Error: %v", clusterName, err)
			return err
		}
		serviceImport := &mcsv1alpha1.ServiceImport{
			ObjectMeta: metav1.ObjectMeta{
				Name:      service.Name,
				Namespace: service.Namespace,
			},
			Spec: mcsv1alpha1.ServiceImportSpec{
				Type: mcsv1alpha1.ClusterSetIP,
				Ports: []mcsv1alpha1.ServicePort{
					{
						Protocol: corev1.ProtocolTCP,
						Port:     80,
					},
				},
			},
		}
		if _, err = leafManager.KosmosClient.MulticlusterV1alpha1().ServiceImports(service.Namespace).Create(ctx, serviceImport, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				klog.Errorf("Create serviceImport in leaf cluster failed %s/%s, Error: %v", service.Namespace, service.Name, err)
				return err
			}
		}
	}
	return nil
}
