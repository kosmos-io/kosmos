package svc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/strings/slices"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	clustertreeutils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const SimpleSyncServiceControllerName = "simple-sync-service-controller"

// SimpleSyncServiceController watches services in root cluster and sync service to leaf cluster directly
type SimpleSyncServiceController struct {
	RootClient              client.Client
	GlobalLeafManager       clustertreeutils.LeafResourceManager
	GlobalLeafClientManager clustertreeutils.LeafClientResourceManager

	// AutoCreateMCSPrefix are the prefix of the namespace for service to auto create in leaf cluster
	AutoCreateMCSPrefix []string
	// ReservedNamespaces are the protected namespaces to prevent Kosmos for deleting system resources
	ReservedNamespaces []string
}

func (c *SimpleSyncServiceController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", SimpleSyncServiceControllerName, request.NamespacedName.String())
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

	// The service is being deleted, in which case we should clear service in all leaf cluster
	if shouldDelete || !service.DeletionTimestamp.IsZero() {
		if err := c.cleanUpServiceInLeafCluster(request.Namespace, request.Name); err != nil {
			klog.Errorf("Cleanup service failed, err: %v", err)
			return controllerruntime.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
		}
		return controllerruntime.Result{}, nil
	}

	err := c.syncServiceInLeafCluster(service)
	if err != nil {
		klog.Errorf("Sync service failed, err: %v", err)
		return controllerruntime.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}
	return controllerruntime.Result{}, nil
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

func (c *SimpleSyncServiceController) shouldEnqueue(service *corev1.Service) bool {
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

func (c *SimpleSyncServiceController) SetupWithManager(mgr manager.Manager) error {
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

			_, ok = updateEvent.ObjectOld.(*corev1.Service)
			if !ok {
				return false
			}

			return c.shouldEnqueue(newService)
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	},
	)

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&corev1.Service{}, servicePredicate).
		Complete(c)
}

func (c *SimpleSyncServiceController) createNamespace(client client.Client, namespace string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	err := client.Create(context.TODO(), ns)
	if err != nil {
		return err
	}
	return nil
}

// nolint:dupl
func (c *SimpleSyncServiceController) cleanUpServiceInLeafCluster(namespace string, name string) error {
	clusters := c.GlobalLeafManager.ListClusters()
	var errs []string
	for _, cluster := range clusters {
		leafClient, err := c.GlobalLeafManager.GetLeafResource(cluster)
		if err != nil {
			klog.Errorf("Failed to get leaf client for cluster %s: %v", cluster, err)
			errs = append(errs, fmt.Sprintf("get leaf client for cluster %s: %v", cluster, err))
			continue
		}

		lcr, err := c.leafClientResource(leafClient)
		if err != nil {
			klog.Errorf("Failed to get leaf client resource %v", leafClient.Cluster.Name)
			errs = append(errs, fmt.Sprintf("get leaf client resource %v", leafClient.Cluster.Name))
			continue
		}

		err = lcr.Clientset.CoreV1().Services(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			klog.Errorf("Failed to delete service %s in cluster %s: %v", name, cluster, err)
			errs = append(errs, fmt.Sprintf("delete service %s in cluster %s: %v", name, cluster, err))
		}
	}

	if len(errs) > 0 {
		return errors.New("errors encountered: " + strings.Join(errs, "; "))
	}
	return nil
}

func (c *SimpleSyncServiceController) syncServiceInLeafCluster(service *corev1.Service) error {
	clusters := c.GlobalLeafClientManager.ListActualClusters()
	errsChan := make(chan string, len(clusters))
	var wg sync.WaitGroup
	for _, cluster := range clusters {
		wg.Add(1)
		go func(cluster string) {
			defer wg.Done()
			leafManager, err := c.GlobalLeafManager.GetLeafResource(cluster)
			if err != nil {
				errsChan <- fmt.Sprintf("get leaf client for cluster %s: %v", cluster, err)
				return
			}

			lcr, err := c.leafClientResource(leafManager)
			if err != nil {
				errsChan <- fmt.Sprintf("get leaf client resource for cluster %s: %v", cluster, err)
				return
			}

			if err = c.createNamespace(lcr.Client, service.Namespace); err != nil && !apierrors.IsAlreadyExists(err) {
				errsChan <- fmt.Sprintf("Create namespace %s in leaf cluster %s failed: %v", service.Namespace, cluster, err)
				return
			}

			err = c.checkServiceType(service, leafManager)
			if err != nil {
				errsChan <- fmt.Sprintf("check service type in leaf cluster %s failed: %v", cluster, err)
				return
			}

			clientService := c.generateService(service, leafManager)
			err = c.createOrUpdateServiceInClient(clientService, leafManager, lcr)
			if err != nil {
				errsChan <- fmt.Sprintf("Create or update service in leaf cluster %s failed: %v", cluster, err)
			}
		}(cluster)
	}
	wg.Wait()
	close(errsChan)

	var errs []string
	for err := range errsChan {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.New("errors encountered: " + strings.Join(errs, "; "))
	}
	return nil
}

func (c *SimpleSyncServiceController) checkServiceType(service *corev1.Service, resource *clustertreeutils.LeafResource) error {
	if *service.Spec.IPFamilyPolicy == corev1.IPFamilyPolicySingleStack {
		if service.Spec.IPFamilies[0] == corev1.IPv6Protocol && resource.IPFamilyType == kosmosv1alpha1.IPFamilyTypeIPV4 ||
			service.Spec.IPFamilies[0] == corev1.IPv4Protocol && resource.IPFamilyType == kosmosv1alpha1.IPFamilyTypeIPV6 {
			return fmt.Errorf("service's IPFamilyPolicy %s is not match the leaf cluster %s", *service.Spec.IPFamilyPolicy, resource.Cluster.Name)
		}
	}
	return nil
}

func (c *SimpleSyncServiceController) generateService(service *corev1.Service, resource *clustertreeutils.LeafResource) *corev1.Service {
	clusterIP := corev1.ClusterIPNone
	if isServiceIPSet(service) {
		clusterIP = ""
	}

	iPFamilies := make([]corev1.IPFamily, 0)
	if resource.IPFamilyType == kosmosv1alpha1.IPFamilyTypeALL {
		iPFamilies = service.Spec.IPFamilies
	} else if resource.IPFamilyType == kosmosv1alpha1.IPFamilyTypeIPV4 {
		iPFamilies = append(iPFamilies, corev1.IPv4Protocol)
	} else {
		iPFamilies = append(iPFamilies, corev1.IPv6Protocol)
	}

	var iPFamilyPolicy corev1.IPFamilyPolicy
	if resource.IPFamilyType == kosmosv1alpha1.IPFamilyTypeALL {
		iPFamilyPolicy = *service.Spec.IPFamilyPolicy
	} else {
		iPFamilyPolicy = corev1.IPFamilyPolicySingleStack
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: service.Namespace,
			Name:      service.Name,
			Annotations: map[string]string{
				utils.ServiceImportLabelKey: utils.MCSLabelValue,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:                  service.Spec.Type,
			ClusterIP:             clusterIP,
			Ports:                 servicePorts(service),
			IPFamilies:            iPFamilies,
			IPFamilyPolicy:        &iPFamilyPolicy,
			ExternalTrafficPolicy: service.Spec.ExternalTrafficPolicy,
		},
	}
}

func (c *SimpleSyncServiceController) createOrUpdateServiceInClient(service *corev1.Service, leafManger *clustertreeutils.LeafResource, leafClient *clustertreeutils.LeafClientResource) error {
	oldService := &corev1.Service{}
	if err := leafClient.Client.Get(context.TODO(), types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, oldService); err != nil {
		if apierrors.IsNotFound(err) {
			if err = leafClient.Client.Create(context.TODO(), service); err != nil {
				klog.Errorf("Create serviceImport service(%s/%s) in client cluster %s failed, Error: %v", service.Namespace, service.Name, leafManger.Cluster.Name, err)
				return err
			}
			return nil
		}
		klog.Errorf("Get service(%s/%s) from in cluster %s failed, Error: %v", service.Namespace, service.Name, leafManger.Cluster.Name, err)
		return err
	}

	retainServiceFields(oldService, service)

	if err := leafClient.Client.Update(context.TODO(), service); err != nil {
		if err != nil {
			klog.Errorf("Update serviceImport service(%s/%s) in cluster %s failed, Error: %v", service.Namespace, service.Name, leafManger.Cluster.Name, err)
			return err
		}
	}
	return nil
}

// nolint:dupl
func isServiceIPSet(service *corev1.Service) bool {
	return service.Spec.ClusterIP != corev1.ClusterIPNone && service.Spec.ClusterIP != ""
}

// nolint:dupl
func servicePorts(service *corev1.Service) []corev1.ServicePort {
	ports := make([]corev1.ServicePort, len(service.Spec.Ports))
	for i, p := range service.Spec.Ports {
		ports[i] = corev1.ServicePort{
			NodePort:    p.NodePort,
			Name:        p.Name,
			Protocol:    p.Protocol,
			Port:        p.Port,
			AppProtocol: p.AppProtocol,
		}
	}
	return ports
}

func (c *SimpleSyncServiceController) leafClientResource(lr *clustertreeutils.LeafResource) (*clustertreeutils.LeafClientResource, error) {
	actualClusterName := clustertreeutils.GetActualClusterName(lr.Cluster)
	lcr, err := c.GlobalLeafClientManager.GetLeafResource(actualClusterName)
	if err != nil {
		return nil, fmt.Errorf("get leaf client resource err: %v", err)
	}
	return lcr, nil
}

// nolint:dupl
func retainServiceFields(oldSvc, newSvc *corev1.Service) {
	newSvc.Spec.ClusterIP = oldSvc.Spec.ClusterIP
	newSvc.ResourceVersion = oldSvc.ResourceVersion
}
