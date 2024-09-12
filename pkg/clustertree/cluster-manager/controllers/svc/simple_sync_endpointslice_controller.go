package svc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
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
	"github.com/kosmos.io/kosmos/pkg/utils/flags"
	"github.com/kosmos.io/kosmos/pkg/utils/helper"
)

const SimpleSyncEPSControllerName = "simple-sync-endpointslice-controller"

// SimpleSyncEPSController watches services in root cluster and sync endpointSlice to leaf cluster directly
type SimpleSyncEPSController struct {
	RootClient              client.Client
	GlobalLeafManager       clustertreeutils.LeafResourceManager
	GlobalLeafClientManager clustertreeutils.LeafClientResourceManager
	// AutoCreateMCSPrefix are the prefix of the namespace for endpointSlice to auto create in leaf cluster
	AutoCreateMCSPrefix []string
	// ReservedNamespaces are the protected namespaces to prevent Kosmos for deleting system resources
	ReservedNamespaces []string
	BackoffOptions     flags.BackoffOptions
}

func (c *SimpleSyncEPSController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", SimpleSyncEPSControllerName, request.NamespacedName.String())
	defer func() {
		klog.V(4).Infof("============ %s has been reconciled =============", request.NamespacedName.String())
	}()

	var shouldDelete bool
	eps := &discoveryv1.EndpointSlice{}
	if err := c.RootClient.Get(ctx, request.NamespacedName, eps); err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Cloud not get endpointSlice in root cluster,Error: %v", err)
			return controllerruntime.Result{Requeue: true}, err
		}
		shouldDelete = true
	}

	// The eps is not found in root cluster, we should delete it in leaf cluster.
	if shouldDelete || !eps.DeletionTimestamp.IsZero() {
		if err := c.cleanUpEpsInLeafCluster(request.Namespace, request.Name); err != nil {
			klog.Errorf("Cleanup MCS resources failed, err: %v", err)
			return controllerruntime.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
		}
		return controllerruntime.Result{}, nil
	}

	serviceName := helper.GetLabelOrAnnotationValue(eps.GetLabels(), utils.ServiceKey)

	service := &corev1.Service{}
	if err := c.RootClient.Get(ctx, types.NamespacedName{Namespace: request.Namespace, Name: serviceName}, service); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Errorf("Service %s/%s not found,ignore it, err: %v", request.Namespace, serviceName, err)
			return controllerruntime.Result{}, nil
		}
		klog.Errorf("Get service %s/%s failed, err: %v", request.Namespace, serviceName, err)
		return controllerruntime.Result{Requeue: true}, err
	}
	if !hasAutoMCSAnnotation(service) && !shouldEnqueueEps(eps, c.AutoCreateMCSPrefix, c.ReservedNamespaces) {
		klog.V(4).Infof("Service %s/%s does not have auto mcs annotation and should not be enqueued, ignore it", request.Namespace, serviceName)
		return controllerruntime.Result{}, nil
	}

	err := c.syncEpsInLeafCluster(eps, serviceName)
	if err != nil {
		return controllerruntime.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}
	return controllerruntime.Result{}, nil
}

func (c *SimpleSyncEPSController) shouldEnqueue(endpointSlice *discoveryv1.EndpointSlice) bool {
	if slices.Contains(c.ReservedNamespaces, endpointSlice.Namespace) {
		return false
	}

	if len(c.AutoCreateMCSPrefix) > 0 {
		for _, prefix := range c.AutoCreateMCSPrefix {
			if strings.HasPrefix(endpointSlice.GetNamespace(), prefix) {
				return true
			}
		}
	}
	return false
}

func shouldEnqueueEps(endpointSlice *discoveryv1.EndpointSlice, autoPrefix, reservedNamespaces []string) bool {
	if slices.Contains(reservedNamespaces, endpointSlice.Namespace) {
		return false
	}

	if len(autoPrefix) > 0 {
		for _, prefix := range autoPrefix {
			if strings.HasPrefix(endpointSlice.GetNamespace(), prefix) {
				return true
			}
		}
	}
	return false
}

func (c *SimpleSyncEPSController) SetupWithManager(mgr manager.Manager) error {
	epsPredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			endpointSlice, ok := event.Object.(*discoveryv1.EndpointSlice)
			if !ok {
				return false
			}

			return c.shouldEnqueue(endpointSlice)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			endpointSlice, ok := deleteEvent.Object.(*discoveryv1.EndpointSlice)
			if !ok {
				return false
			}

			return c.shouldEnqueue(endpointSlice)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			newEps, ok := updateEvent.ObjectNew.(*discoveryv1.EndpointSlice)
			if !ok {
				return false
			}

			_, ok = updateEvent.ObjectOld.(*discoveryv1.EndpointSlice)
			if !ok {
				return false
			}

			return c.shouldEnqueue(newEps)
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	},
	)

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&discoveryv1.EndpointSlice{}, epsPredicate).
		Complete(c)
}

func (c *SimpleSyncEPSController) createNamespace(client client.Client, namespace string) error {
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
func (c *SimpleSyncEPSController) cleanUpEpsInLeafCluster(namespace string, name string) error {
	clusters := c.GlobalLeafClientManager.ListActualClusters()
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

		err = lcr.Clientset.DiscoveryV1().EndpointSlices(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			klog.Errorf("Failed to delete endpointSlice %s in cluster %s: %v", name, cluster, err)
			errs = append(errs, fmt.Sprintf("delete endpointSlice %s in cluster %s: %v", name, cluster, err))
		}
	}

	if len(errs) > 0 {
		return errors.New("errors encountered: " + strings.Join(errs, "; "))
	}
	return nil
}

func (c *SimpleSyncEPSController) syncEpsInLeafCluster(eps *discoveryv1.EndpointSlice, serviceName string) error {
	endpointSlice := eps.DeepCopy()

	clusters := c.GlobalLeafManager.ListClusters()
	errsChan := make(chan string, len(clusters))
	var wg sync.WaitGroup
	for _, cluster := range clusters {
		wg.Add(1)
		go func(cluster, serviceName string) {
			defer wg.Done()
			leafManager, err := c.GlobalLeafManager.GetLeafResource(cluster)
			if err != nil {
				errsChan <- fmt.Sprintf("get leaf client for cluster %s: %v", cluster, err)
				return
			}

			lcr, err := c.leafClientResource(leafManager)
			if err != nil {
				klog.Errorf("Failed to get leaf client resource %v", leafManager.Cluster.Name)
				errsChan <- fmt.Sprintf("get leaf client resource %v", leafManager.Cluster.Name)
				return
			}

			if endpointSlice.AddressType == discoveryv1.AddressTypeIPv4 && leafManager.IPFamilyType == kosmosv1alpha1.IPFamilyTypeIPV6 ||
				endpointSlice.AddressType == discoveryv1.AddressTypeIPv6 && leafManager.IPFamilyType == kosmosv1alpha1.IPFamilyTypeIPV4 {
				klog.Warningf("The endpointSlice's AddressType is not match leaf cluster %s IPFamilyType,so ignore it", cluster)
				klog.Errorf("The endpointSlice's AddressType is not match leaf cluster %s IPFamilyType,so ignore it", cluster)
				return
			}

			if err = c.createNamespace(lcr.Client, endpointSlice.Namespace); err != nil && !apierrors.IsAlreadyExists(err) {
				errsChan <- fmt.Sprintf("Create namespace %s in leaf cluster %s failed: %v", endpointSlice.Namespace, cluster, err)
				return
			}

			newSlice := retainEndpointSlice(endpointSlice, serviceName)

			if err = lcr.Client.Create(context.TODO(), newSlice); err != nil {
				if apierrors.IsAlreadyExists(err) {
					if err = c.updateEndpointSlice(newSlice, lcr); err != nil {
						errsChan <- fmt.Sprintf("Update endpointSlice %s in leaf cluster %s failed: %v", newSlice.Name, cluster, err)
						return
					}
				} else {
					errsChan <- fmt.Sprintf("Create endpointSlice %s in leaf cluster %s failed: %v", newSlice.Name, cluster, err)
					return
				}
			}
		}(cluster, serviceName)
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

func (c *SimpleSyncEPSController) updateEndpointSlice(slice *discoveryv1.EndpointSlice, leafManager *clustertreeutils.LeafClientResource) error {
	eps := slice.DeepCopy()
	return retry.RetryOnConflict(flags.DefaultUpdateRetryBackoff(c.BackoffOptions), func() error {
		updateErr := leafManager.Client.Update(context.TODO(), eps)
		if apierrors.IsNotFound(updateErr) {
			return nil
		}
		if updateErr == nil {
			return nil
		}
		klog.Errorf("Failed to update endpointSlice %s/%s: %v", eps.Namespace, eps.Name, updateErr)
		newEps := &discoveryv1.EndpointSlice{}
		getErr := leafManager.Client.Get(context.TODO(), client.ObjectKey{Namespace: eps.Namespace, Name: eps.Name}, newEps)
		if getErr == nil {
			//Make a copy, so we don't mutate the shared cache
			eps = newEps.DeepCopy()
		} else {
			if apierrors.IsNotFound(getErr) {
				return nil
			}
			klog.Errorf("Failed to get updated endpointSlice %s/%s: %v", eps.Namespace, eps.Name, getErr)
		}

		return updateErr
	})
}

func (c *SimpleSyncEPSController) leafClientResource(lr *clustertreeutils.LeafResource) (*clustertreeutils.LeafClientResource, error) {
	actualClusterName := clustertreeutils.GetActualClusterName(lr.Cluster)
	lcr, err := c.GlobalLeafClientManager.GetLeafResource(actualClusterName)
	if err != nil {
		return nil, fmt.Errorf("get leaf client resource err: %v", err)
	}
	return lcr, nil
}

func retainEndpointSlice(original *discoveryv1.EndpointSlice, serviceName string) *discoveryv1.EndpointSlice {
	endpointSlice := original.DeepCopy()
	endpointSlice.ObjectMeta = metav1.ObjectMeta{
		Namespace: original.Namespace,
		Name:      original.Name,
	}
	helper.AddEndpointSliceLabel(endpointSlice, utils.ServiceKey, serviceName)
	return endpointSlice
}
