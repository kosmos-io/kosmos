package mcs

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/helper"
	"github.com/kosmos.io/kosmos/pkg/utils/keys"
)

const LeafServiceImportControllerName = "leaf-service-import-controller"

// ServiceImportController watches serviceImport in leaf node and sync service and endpointSlice in root cluster
type ServiceImportController struct {
	LeafClient          client.Client
	RootKosmosClient    kosmosversioned.Interface
	LeafNodeName        string
	IPFamilyType        kosmosv1alpha1.IPFamilyType
	EventRecorder       record.EventRecorder
	Logger              logr.Logger
	processor           utils.AsyncWorker
	RootResourceManager *utils.ResourceManager
	ctx                 context.Context
	// ReservedNamespaces are the protected namespaces to prevent Kosmos for deleting system resources
	ReservedNamespaces []string
}

func (c *ServiceImportController) AddController(mgr manager.Manager) error {
	if err := mgr.Add(c); err != nil {
		klog.Errorf("Unable to create %s Error: %v", LeafServiceImportControllerName, err)
	}
	return nil
}

func (c *ServiceImportController) Start(ctx context.Context) error {
	klog.Infof("Starting %s", LeafServiceImportControllerName)
	defer klog.Infof("Stop %s as process done.", LeafServiceImportControllerName)

	opt := utils.Options{
		Name: LeafServiceImportControllerName,
		KeyFunc: func(obj interface{}) (utils.QueueKey, error) {
			// Don't care about the GVK in the queue
			return keys.NamespaceWideKeyFunc(obj)
		},
		ReconcileFunc: c.Reconcile,
	}
	c.processor = utils.NewAsyncWorker(opt)
	c.ctx = ctx

	serviceImportInformerFactory := externalversions.NewSharedInformerFactory(c.RootKosmosClient, 0)
	serviceImportInformer := serviceImportInformerFactory.Multicluster().V1alpha1().ServiceImports()
	_, err := serviceImportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.OnAdd,
		UpdateFunc: c.OnUpdate,
		DeleteFunc: c.OnDelete,
	})
	if err != nil {
		return err
	}

	_, err = c.RootResourceManager.EndpointSliceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.OnEpsAdd,
		UpdateFunc: c.OnEpsUpdate,
		DeleteFunc: c.OnEpsDelete,
	})
	if err != nil {
		return err
	}

	stopCh := ctx.Done()
	serviceImportInformerFactory.Start(stopCh)
	serviceImportInformerFactory.WaitForCacheSync(stopCh)

	c.processor.Run(utils.DefaultWorkers, stopCh)
	<-stopCh
	return nil
}

func (c *ServiceImportController) Reconcile(key utils.QueueKey) error {
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Error("invalid key")
		return fmt.Errorf("invalid key")
	}
	klog.V(4).Infof("============ %s starts to reconcile %s in cluster %s ============", LeafServiceImportControllerName, clusterWideKey.NamespaceKey(), c.LeafNodeName)
	defer func() {
		klog.V(4).Infof("============ %s has been reconciled in cluster %s =============", clusterWideKey.NamespaceKey(), c.LeafNodeName)
	}()

	var shouldDelete bool
	serviceImport := &mcsv1alpha1.ServiceImport{}
	if err := c.LeafClient.Get(c.ctx, types.NamespacedName{Namespace: clusterWideKey.Namespace, Name: clusterWideKey.Name}, serviceImport); err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Get %s in cluster %s failed, Error: %v", clusterWideKey.NamespaceKey(), c.LeafNodeName, err)
			return err
		}
		shouldDelete = true
	}

	// The serviceImport is being deleted, in which case we should clear endpointSlice.
	if shouldDelete || !serviceImport.DeletionTimestamp.IsZero() {
		if err := c.cleanupServiceAndEndpointSlice(c.ctx, clusterWideKey.Namespace, clusterWideKey.Name); err != nil {
			return err
		}
		return nil
	}

	err := c.syncServiceImport(c.ctx, serviceImport)
	if err != nil {
		return err
	}
	return nil
}

func (c *ServiceImportController) cleanupServiceAndEndpointSlice(ctx context.Context, namespace, name string) error {
	service := &corev1.Service{}
	if err := c.LeafClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, service); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("ServiceImport %s/%s is deleting and Service %s/%s is not found, ignore it", namespace, name, namespace, name)
			return nil
		}
		klog.Errorf("ServiceImport %s/%s is deleting but clean up service in cluster %s failed, Error: %v", namespace, name, c.LeafNodeName, err)
		return err
	}

	if !helper.HasAnnotation(service.ObjectMeta, utils.ServiceImportLabelKey) {
		klog.V(4).Infof("Service %s/%s is not managed by kosmos, ignore it", namespace, name)
		return nil
	}

	if err := c.LeafClient.Delete(ctx, service); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("ServiceImport %s/%s is deleting and Service %s/%s is not found, ignore it", namespace, name, namespace, name)
			return nil
		}
		klog.Errorf("ServiceImport %s/%s is deleting but clean up service in cluster %s failed, Error: %v", namespace, name, c.LeafNodeName, err)
		return err
	}

	endpointSlice := &discoveryv1.EndpointSlice{}
	err := c.LeafClient.DeleteAllOf(ctx, endpointSlice, &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			Namespace: namespace,
			LabelSelector: labels.SelectorFromSet(map[string]string{
				utils.ServiceKey: name,
			}),
		},
	})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("ServiceImport %s/%s is deleting and Service %s/%s is not found, ignore it", namespace, name, namespace, name)
			return nil
		}
		klog.Errorf("ServiceImport %s/%s is deleting but clean up service in cluster %s failed, Error: %v", namespace, name, c.LeafNodeName, err)
		return err
	}
	return nil
}

func (c *ServiceImportController) syncServiceImport(ctx context.Context, serviceImport *mcsv1alpha1.ServiceImport) error {
	rootService, err := c.RootResourceManager.ServiceLister.Services(serviceImport.Namespace).Get(serviceImport.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("Service %s/%s is not found in root cluster, ignore it", serviceImport.Namespace, serviceImport.Name)
			return nil
		}
		klog.Errorf("Get Service %s/%s failed from root cluster", serviceImport.Namespace, serviceImport.Name, err)
		return err
	}

	if err := c.importServiceHandler(ctx, rootService, serviceImport); err != nil {
		klog.Errorf("Create or update service %s/%s in client cluster %s failed, error: %v", serviceImport.Namespace, serviceImport.Name, c.LeafNodeName, err)
		return err
	}

	epsList, err := c.RootResourceManager.EndpointSliceLister.EndpointSlices(serviceImport.Namespace).List(labels.SelectorFromSet(map[string]string{utils.ServiceKey: serviceImport.Name}))
	if err != nil {
		klog.Errorf("Get endpointSlices in namespace %s from cluster %s failed, error: %v", serviceImport.Namespace, err)
		return err
	}

	addresses := make([]string, 0)
	for _, eps := range epsList {
		epsCopy := eps.DeepCopy()
		for _, endpoint := range epsCopy.Endpoints {
			for _, address := range endpoint.Addresses {
				newAddress := address
				addresses = append(addresses, newAddress)
			}
		}
		err = c.importEndpointSliceHandler(ctx, epsCopy, serviceImport)
		if err != nil {
			klog.Errorf("Create or update service %s/%s in client cluster failed, error: %v", serviceImport.Namespace, serviceImport.Name, err)
			return err
		}
	}

	addressString := strings.Join(addresses, ",")
	helper.AddServiceImportAnnotation(serviceImport, utils.ServiceEndpointsKey, addressString)
	if err = c.updateServiceImport(ctx, serviceImport, addressString); err != nil {
		klog.Errorf("Update serviceImport (%s/%s) annotation in cluster %s failed, Error: %v", serviceImport.Namespace, serviceImport.Name, c.LeafNodeName, err)
		return err
	}

	c.EventRecorder.Event(serviceImport, corev1.EventTypeNormal, "Synced", "serviceImport has been synced successfully")
	return nil
}

func (c *ServiceImportController) importEndpointSliceHandler(ctx context.Context, endpointSlice *discoveryv1.EndpointSlice, serviceImport *mcsv1alpha1.ServiceImport) error {
	if metav1.HasAnnotation(serviceImport.ObjectMeta, utils.DisconnectedEndpointsKey) {
		annotationValue := helper.GetLabelOrAnnotationValue(serviceImport.Annotations, utils.DisconnectedEndpointsKey)
		disConnectedAddress := strings.Split(annotationValue, ",")
		clearEndpointSlice(endpointSlice, disConnectedAddress)
	}

	if endpointSlice.AddressType == discoveryv1.AddressTypeIPv4 && c.IPFamilyType == kosmosv1alpha1.IPFamilyTypeIPV6 ||
		endpointSlice.AddressType == discoveryv1.AddressTypeIPv6 && c.IPFamilyType == kosmosv1alpha1.IPFamilyTypeIPV4 {
		klog.Warningf("The endpointSlice's AddressType is not match leaf cluster %s IPFamilyType,so ignore it", c.LeafNodeName)
		return nil
	}

	return c.createOrUpdateEndpointSliceInClient(ctx, endpointSlice, serviceImport.Name)
}

func (c *ServiceImportController) createOrUpdateEndpointSliceInClient(ctx context.Context, endpointSlice *discoveryv1.EndpointSlice, serviceName string) error {
	newSlice := retainEndpointSlice(endpointSlice, serviceName)

	if err := c.LeafClient.Create(ctx, newSlice); err != nil {
		if apierrors.IsAlreadyExists(err) {
			err = c.updateEndpointSlice(ctx, newSlice)
			if err != nil {
				klog.Errorf("Update endpointSlice(%s/%s) in cluster %s failed, Error: %v", newSlice.Namespace, newSlice.Name, c.LeafNodeName, err)
				return err
			}
			return nil
		}
		klog.Errorf("Create endpointSlice(%s/%s) in cluster %s failed, Error: %v", newSlice.Namespace, newSlice.Name, c.LeafNodeName, err)
		return err
	}
	return nil
}

func (c *ServiceImportController) updateEndpointSlice(ctx context.Context, endpointSlice *discoveryv1.EndpointSlice) error {
	newEps := endpointSlice.DeepCopy()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		updateErr := c.LeafClient.Update(ctx, newEps)
		if updateErr == nil {
			return nil
		}

		updated := &discoveryv1.EndpointSlice{}
		getErr := c.LeafClient.Get(ctx, types.NamespacedName{Namespace: newEps.Namespace, Name: newEps.Name}, updated)
		if getErr == nil {
			//Make a copy, so we don't mutate the shared cache
			newEps = updated.DeepCopy()
		} else {
			klog.Errorf("Failed to get updated endpointSlice %s/%s in cluster %s: %v", endpointSlice.Namespace, endpointSlice.Name, c.LeafNodeName, getErr)
		}

		return updateErr
	})
}

func retainEndpointSlice(original *discoveryv1.EndpointSlice, serviceName string) *discoveryv1.EndpointSlice {
	endpointSlice := original.DeepCopy()
	endpointSlice.ObjectMeta = metav1.ObjectMeta{
		Namespace: original.Namespace,
		Name:      original.Name,
	}
	helper.AddEndpointSliceAnnotation(endpointSlice, utils.ServiceImportLabelKey, utils.MCSLabelValue)
	helper.AddEndpointSliceLabel(endpointSlice, utils.ServiceKey, serviceName)
	return endpointSlice
}

func clearEndpointSlice(slice *discoveryv1.EndpointSlice, disconnectedAddress []string) {
	disconnectedAddressMap := make(map[string]struct{})
	for _, name := range disconnectedAddress {
		disconnectedAddressMap[name] = struct{}{}
	}

	endpoints := slice.Endpoints
	newEndpoints := make([]discoveryv1.Endpoint, 0)
	for _, endpoint := range endpoints {
		newAddresses := make([]string, 0)
		for _, address := range endpoint.Addresses {
			if _, found := disconnectedAddressMap[address]; !found {
				newAddresses = append(newAddresses, address)
			}
		}
		// Only add non-empty addresses from endpoints
		if len(newAddresses) > 0 {
			endpoint.Addresses = newAddresses
			newEndpoints = append(newEndpoints, endpoint)
		}
	}
	slice.Endpoints = newEndpoints
}

func (c *ServiceImportController) importServiceHandler(ctx context.Context, rootService *corev1.Service, serviceImport *mcsv1alpha1.ServiceImport) error {
	err := c.checkServiceType(rootService)
	if err != nil {
		klog.Warningf("Cloud not create service in leaf cluster %s,Error: %v", c.LeafNodeName, err)
		// return nil will not requeue
		return nil
	}
	clientService := c.generateService(rootService, serviceImport)
	err = c.createOrUpdateServiceInClient(ctx, clientService)
	if err != nil {
		return err
	}
	return nil
}

func (c *ServiceImportController) createOrUpdateServiceInClient(ctx context.Context, service *corev1.Service) error {
	oldService := &corev1.Service{}
	if err := c.LeafClient.Get(ctx, types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, oldService); err != nil {
		if apierrors.IsNotFound(err) {
			if err = c.LeafClient.Create(ctx, service); err != nil {
				klog.Errorf("Create serviceImport service(%s/%s) in client cluster %s failed, Error: %v", service.Namespace, service.Name, c.LeafNodeName, err)
				return err
			} else {
				return nil
			}
		}
		klog.Errorf("Get service(%s/%s) from in cluster %s failed, Error: %v", service.Namespace, service.Name, c.LeafNodeName, err)
		return err
	}

	retainServiceFields(oldService, service)

	if err := c.LeafClient.Update(ctx, service); err != nil {
		if err != nil {
			klog.Errorf("Update serviceImport service(%s/%s) in cluster %s failed, Error: %v", service.Namespace, service.Name, c.LeafNodeName, err)
			return err
		}
	}
	return nil
}

func (c *ServiceImportController) updateServiceImport(ctx context.Context, serviceImport *mcsv1alpha1.ServiceImport, addresses string) error {
	newImport := serviceImport.DeepCopy()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		updateErr := c.LeafClient.Update(ctx, newImport)
		if updateErr == nil {
			return nil
		}
		updated := &mcsv1alpha1.ServiceImport{}
		getErr := c.LeafClient.Get(ctx, types.NamespacedName{Namespace: newImport.Namespace, Name: newImport.Name}, updated)
		if getErr == nil {
			// Make a copy, so we don't mutate the shared cache
			newImport = updated.DeepCopy()
			helper.AddServiceImportAnnotation(newImport, utils.ServiceEndpointsKey, addresses)
		} else {
			klog.Errorf("Failed to get updated serviceImport %s/%s in cluster %s,Error : %v", newImport.Namespace, serviceImport.Name, c.LeafNodeName, getErr)
		}
		return updateErr
	})
}

func (c *ServiceImportController) OnAdd(obj interface{}) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return
	}
	c.processor.Enqueue(runtimeObj)
}

func (c *ServiceImportController) OnUpdate(old interface{}, new interface{}) {
	runtimeObj, ok := new.(runtime.Object)
	if !ok {
		return
	}
	c.processor.Enqueue(runtimeObj)
}

func (c *ServiceImportController) OnDelete(obj interface{}) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return
	}
	c.processor.Enqueue(runtimeObj)
}

func (c *ServiceImportController) OnEpsAdd(obj interface{}) {
	eps := obj.(*discoveryv1.EndpointSlice)
	if !c.shouldEnqueue(eps) {
		return
	}

	if helper.HasAnnotation(eps.ObjectMeta, utils.ServiceExportLabelKey) {
		serviceExportName := helper.GetLabelOrAnnotationValue(eps.GetLabels(), utils.ServiceKey)
		key := keys.ClusterWideKey{}
		key.Namespace = eps.Namespace
		key.Name = serviceExportName
		c.processor.Add(key)
	}
}

func (c *ServiceImportController) OnEpsUpdate(old interface{}, new interface{}) {
	newSlice := new.(*discoveryv1.EndpointSlice)
	oldSlice := old.(*discoveryv1.EndpointSlice)
	if !c.shouldEnqueue(newSlice) {
		return
	}

	isRemoveAnnotationEvent := helper.HasAnnotation(oldSlice.ObjectMeta, utils.ServiceExportLabelKey) && !helper.HasAnnotation(newSlice.ObjectMeta, utils.ServiceExportLabelKey)
	if helper.HasAnnotation(newSlice.ObjectMeta, utils.ServiceExportLabelKey) || isRemoveAnnotationEvent {
		serviceExportName := helper.GetLabelOrAnnotationValue(newSlice.GetLabels(), utils.ServiceKey)
		key := keys.ClusterWideKey{}
		key.Namespace = newSlice.Namespace
		key.Name = serviceExportName
		c.processor.Add(key)
	}
}

func (c *ServiceImportController) OnEpsDelete(obj interface{}) {
	eps := obj.(*discoveryv1.EndpointSlice)
	if !c.shouldEnqueue(eps) {
		return
	}

	if helper.HasAnnotation(eps.ObjectMeta, utils.ServiceExportLabelKey) {
		serviceExportName := helper.GetLabelOrAnnotationValue(eps.GetLabels(), utils.ServiceKey)
		key := keys.ClusterWideKey{}
		key.Namespace = eps.Namespace
		key.Name = serviceExportName
		c.processor.Add(key)
	}
}

func (c *ServiceImportController) shouldEnqueue(endpointSlice *discoveryv1.EndpointSlice) bool {
	return !slices.Contains(c.ReservedNamespaces, endpointSlice.Namespace)
}

func retainServiceFields(oldSvc, newSvc *corev1.Service) {
	newSvc.Spec.ClusterIP = oldSvc.Spec.ClusterIP
	newSvc.ResourceVersion = oldSvc.ResourceVersion
}

func (c *ServiceImportController) generateService(service *corev1.Service, serviceImport *mcsv1alpha1.ServiceImport) *corev1.Service {
	clusterIP := corev1.ClusterIPNone
	if isServiceIPSet(service) {
		clusterIP = ""
	}

	iPFamilies := make([]corev1.IPFamily, 0)
	if c.IPFamilyType == kosmosv1alpha1.IPFamilyTypeALL {
		iPFamilies = service.Spec.IPFamilies
	} else if c.IPFamilyType == kosmosv1alpha1.IPFamilyTypeIPV4 {
		iPFamilies = append(iPFamilies, corev1.IPv4Protocol)
	} else {
		iPFamilies = append(iPFamilies, corev1.IPv6Protocol)
	}

	var iPFamilyPolicy corev1.IPFamilyPolicy
	if c.IPFamilyType == kosmosv1alpha1.IPFamilyTypeALL {
		iPFamilyPolicy = *service.Spec.IPFamilyPolicy
	} else {
		iPFamilyPolicy = corev1.IPFamilyPolicySingleStack
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: serviceImport.Namespace,
			Name:      service.Name,
			Annotations: map[string]string{
				utils.ServiceImportLabelKey: utils.MCSLabelValue,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:           service.Spec.Type,
			ClusterIP:      clusterIP,
			Ports:          servicePorts(service),
			IPFamilies:     iPFamilies,
			IPFamilyPolicy: &iPFamilyPolicy,
		},
	}
}

func (c *ServiceImportController) checkServiceType(service *corev1.Service) error {
	if *service.Spec.IPFamilyPolicy == corev1.IPFamilyPolicySingleStack {
		if service.Spec.IPFamilies[0] == corev1.IPv6Protocol && c.IPFamilyType == kosmosv1alpha1.IPFamilyTypeIPV4 ||
			service.Spec.IPFamilies[0] == corev1.IPv4Protocol && c.IPFamilyType == kosmosv1alpha1.IPFamilyTypeIPV6 {
			return fmt.Errorf("service's IPFamilyPolicy %s is not match the leaf cluster %s", *service.Spec.IPFamilyPolicy, c.LeafNodeName)
		}
	}
	return nil
}

func isServiceIPSet(service *corev1.Service) bool {
	return service.Spec.ClusterIP != corev1.ClusterIPNone && service.Spec.ClusterIP != ""
}

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
