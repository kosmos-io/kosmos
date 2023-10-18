package mcs

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	discoveryv1listers "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/controllers"
	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/utils/podutils"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	kosmosinformer "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	kosmoslisters "github.com/kosmos.io/kosmos/pkg/generated/listers/apis/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/utils/helper"
)

const (
	ServiceKey                  = "kubernetes.io/service-name"
	ServiceImportControllerName = "serviceimport-controller"
	ServiceExportLabelKey       = "kosmos.io/service-export"
	ServiceImportLabelKey       = "kosmos.io/service-import"
	MCSLabelValue               = "ture"
	ConnectedEndpointsKey       = "kosmos.io/connected-address"
	DisconnectedEndpointsKey    = "kosmos.io/disconnected-address"
)

// ServiceImportController is to sync serviceImport and synchronize labeled endpointSlices
// and services to the member clusters according to ServiceImport
type ServiceImportController struct {
	client       kubernetes.Interface
	kosmosClient kosmosversioned.Interface

	clientEventRecorder               record.EventRecorder
	clientServiceImportQueue          workqueue.RateLimitingInterface
	clientEndpointSliceQueue          workqueue.RateLimitingInterface
	clientServiceLister               corev1listers.ServiceLister
	clientServiceInformerSynced       cache.InformerSynced
	clientEndpointSliceLister         discoveryv1listers.EndpointSliceLister
	clientEndpointSliceInformerSynced cache.InformerSynced
	clientServiceImportLister         kosmoslisters.ServiceImportLister
	clientServiceImportInformerSynced cache.InformerSynced

	masterServiceLister               corev1listers.ServiceLister
	masterServiceInformerSynced       cache.InformerSynced
	masterEndpointSliceLister         discoveryv1listers.EndpointSliceLister
	masterEndpointSliceInformerSynced cache.InformerSynced
}

// NewServiceImportController create a new serviceImport controller
func NewServiceImportController(client kubernetes.Interface, kosmosClient kosmosversioned.Interface, clientInformer, masterInformer informers.SharedInformerFactory, kosmosClientInformer kosmosinformer.SharedInformerFactory) (*ServiceImportController, error) {
	c := &ServiceImportController{
		client:                   client,
		kosmosClient:             kosmosClient,
		clientServiceImportQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		clientEndpointSliceQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	masterServiceInformer := masterInformer.Core().V1().Services()
	masterEndpointSliceInformer := masterInformer.Discovery().V1().EndpointSlices()

	c.masterServiceLister = masterServiceInformer.Lister()
	c.masterServiceInformerSynced = masterServiceInformer.Informer().HasSynced
	c.masterEndpointSliceLister = masterEndpointSliceInformer.Lister()
	c.masterEndpointSliceInformerSynced = masterEndpointSliceInformer.Informer().HasSynced

	_, err := masterEndpointSliceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.masterEndpointSliceAdded,
		UpdateFunc: c.masterEndpointSliceUpdated,
		DeleteFunc: c.masterEndpointSliceDeleted,
	})
	if err != nil {
		return nil, err
	}

	clientBroadcaster := record.NewBroadcaster()
	clientBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: client.CoreV1().Events(v1.NamespaceAll)})
	clientEventRecorder := clientBroadcaster.NewRecorder(scheme.NewSchema(), v1.EventSource{Component: controllers.ComponentName})
	c.clientEventRecorder = clientEventRecorder

	clientServiceInformer := clientInformer.Core().V1().Services()
	clientEndpointSliceInformer := clientInformer.Discovery().V1().EndpointSlices()
	clientServiceImportInformer := kosmosClientInformer.Multicluster().V1alpha1().ServiceImports()

	c.clientServiceLister = clientServiceInformer.Lister()
	c.clientServiceInformerSynced = clientServiceInformer.Informer().HasSynced
	c.clientEndpointSliceLister = clientEndpointSliceInformer.Lister()
	c.clientEndpointSliceInformerSynced = clientEndpointSliceInformer.Informer().HasSynced
	c.clientServiceImportLister = clientServiceImportInformer.Lister()
	c.clientServiceImportInformerSynced = clientServiceImportInformer.Informer().HasSynced

	_, err = clientServiceImportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.clientServiceImportAdded,
		UpdateFunc: c.clientServiceImportUpdated,
		DeleteFunc: c.clientServiceImportDeleted,
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *ServiceImportController) Run(ctx context.Context) error {
	defer c.clientEndpointSliceQueue.ShutDown()
	defer c.clientServiceImportQueue.ShutDown()

	klog.Infof("Starting %s", ServiceImportControllerName)
	defer klog.Infof("Shutting %s", ServiceImportControllerName)

	stopCh := ctx.Done()
	workers := 1

	if !cache.WaitForCacheSync(stopCh, c.clientServiceImportInformerSynced, c.clientEndpointSliceInformerSynced, c.clientServiceInformerSynced) {
		return fmt.Errorf("cannot sync endpointSlice serviceImport service caches from client")
	}
	klog.Infof("Client endpointSlice serviceImport service caches synced")

	for i := 0; i < workers; i++ {
		go wait.Until(c.syncClientEndpointSlice, 0, stopCh)
		go wait.Until(c.syncClientServiceImport, 0, stopCh)
	}

	<-stopCh
	return nil
}

func (c *ServiceImportController) masterEndpointSliceAdded(obj interface{}) {
	eps := obj.(*discoveryv1.EndpointSlice)
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	klog.Info("Enqueue endpointSlice in master add ", "key ", key)
	if c.shouldEnqueueEndpointSlice(eps.ObjectMeta) {
		klog.Info("Enqueue endpointSlice in knode add ", "key ", key)
		c.clientEndpointSliceQueue.Add(key)
	}
}

func (c *ServiceImportController) masterEndpointSliceUpdated(old, new interface{}) {
	newSlice := new.(*discoveryv1.EndpointSlice)
	oldSlice := old.(*discoveryv1.EndpointSlice)
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	if c.shouldEnqueueEndpointSlice(newSlice.ObjectMeta) || (helper.HasAnnotation(oldSlice.ObjectMeta, ServiceExportLabelKey) && !helper.HasAnnotation(newSlice.ObjectMeta, ServiceExportLabelKey)) {
		klog.Info("Enqueue endpointSlice in knode add ", "key ", key)
		c.clientEndpointSliceQueue.Add(key)
	}
}

func (c *ServiceImportController) masterEndpointSliceDeleted(obj interface{}) {
	eps := obj.(*discoveryv1.EndpointSlice)
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	if c.shouldEnqueueEndpointSlice(eps.ObjectMeta) {
		klog.Info("Enqueue endpointSlice in knode add", "key ", key)
		c.clientEndpointSliceQueue.Add(key)
	}
}

func (c *ServiceImportController) clientServiceImportAdded(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Client serviceImport add ", "key ", key)
	c.clientServiceImportQueue.Add(key)
}

func (c *ServiceImportController) clientServiceImportUpdated(old, new interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Client serviceImport update ", "key ", key)
	c.clientServiceImportQueue.Add(key)
}

func (c *ServiceImportController) clientServiceImportDeleted(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Client serviceImport delete ", "key ", key)
	c.clientServiceImportQueue.Add(key)
}

func (c *ServiceImportController) updateEndpointSlice(endpointSlice *discoveryv1.EndpointSlice, client kubernetes.Interface) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, updateErr := client.DiscoveryV1().EndpointSlices(endpointSlice.Namespace).Update(context.TODO(), endpointSlice, metav1.UpdateOptions{})
		if updateErr == nil {
			return nil
		}

		updated, getErr := client.DiscoveryV1().EndpointSlices(endpointSlice.Namespace).Get(context.TODO(), endpointSlice.Name, metav1.GetOptions{})
		if getErr == nil {
			//Make a copy, so we don't mutate the shared cache
			endpointSlice = updated.DeepCopy()
		} else {
			klog.Errorf("Failed to get updated endpointSlice %s/%s: %v", endpointSlice.Namespace, endpointSlice.Name, getErr)
		}

		return updateErr
	})
}

func (c *ServiceImportController) syncClientEndpointSlice() {
	keyObj, quit := c.clientEndpointSliceQueue.Get()
	if quit {
		return
	}
	defer c.clientEndpointSliceQueue.Done(keyObj)

	key := keyObj.(string)
	namespace, sliceName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		err = nil
		return
	}
	klog.V(4).Infof("Started client endpointSlice processing %s/%s", namespace, sliceName)

	defer func() {
		if err != nil {
			c.clientEndpointSliceQueue.AddRateLimited(keyObj)
		}
		c.clientEndpointSliceQueue.Forget(keyObj)
	}()

	needsCleanup := false
	masterSlice, err := c.masterEndpointSliceLister.EndpointSlices(namespace).Get(sliceName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Get endpointSlice in master cluster failed, error: %v", err)
			return
		}
		err = nil
		needsCleanup = true
		klog.V(3).Infof("EndpointSlice %/%s deleted", namespace, sliceName)
	}

	if !helper.HasAnnotation(masterSlice.ObjectMeta, ServiceExportLabelKey) {
		needsCleanup = true
	}

	if needsCleanup || masterSlice.DeletionTimestamp != nil {
		err := c.cleanupEndpointSliceInClient(namespace, sliceName)
		if err != nil {
			klog.Errorf("Cleanup endpointSlice in client cluster failed, error: %v", err)
			return
		}
		return
	}

	serviceImportName := helper.GetLabelOrAnnotationValue(masterSlice.Labels, ServiceKey)
	serviceImport, err := c.clientServiceImportLister.ServiceImports(namespace).Get(serviceImportName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Get serviceImport from client cluster failed, error: %v", err)
			return
		}
		err = nil
		klog.V(3).Infof("EndpointSlice %s/%s does not have a serviceImport in client, should not be synced", namespace, sliceName)
		return
	}

	err = c.importEndpointSliceHandler(masterSlice, serviceImport)
	if err != nil {
		klog.Errorf("Create or update endpointSlice %/%s in client cluster failed, error: %v", namespace, sliceName, err)
		return
	}

	c.clientEventRecorder.Event(masterSlice, v1.EventTypeNormal, "Synced", "endpointSlice has been synced successfully")
	klog.V(4).Infof("Handler endpointSlice: finished processing %s/%s", namespace, sliceName)
}

func (c *ServiceImportController) syncClientServiceImport() {
	keyObj, quit := c.clientServiceImportQueue.Get()
	if quit {
		return
	}
	defer c.clientServiceImportQueue.Done(keyObj)

	key := keyObj.(string)
	namespace, importName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		err = nil
		return
	}
	klog.V(4).Infof("Started serviceImport processing %s/%s", namespace, importName)

	defer func() {
		if err != nil {
			c.clientServiceImportQueue.AddRateLimited(keyObj)
		}
		c.clientServiceImportQueue.Forget(keyObj)
	}()

	shouldCleanup := false
	serviceImport, err := c.clientServiceImportLister.ServiceImports(namespace).Get(importName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Get serviceImport from client cluster failed, error: %v", err)
			return
		}
		err = nil
		shouldCleanup = true
		klog.V(3).Infof("ServiceImport %s/%s deleted", namespace, importName)
	}

	if shouldCleanup || serviceImport.DeletionTimestamp != nil {
		err := c.cleanupServiceAndEndpointSlice(namespace, importName)
		if err != nil {
			klog.Errorf("Cleanup service and endpointSlice in client cluster failed, error: %v", err)
			return
		}
		klog.V(3).Infof("ServiceImport %q deleted", importName)
		return
	}

	masterService, err := c.masterServiceLister.Services(namespace).Get(importName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Get service from master cluster failed, error: %v", err)
			return
		}
		err = nil
		klog.V(3).Infof("Service %s/%s deleted in master", namespace, importName)
		return
	}

	err = c.importServiceHandler(masterService, serviceImport)
	if err != nil {
		if apierrors.IsInvalid(err) {
			klog.Errorf("Create or update service %s/%s in client cluster invalid, error: %v", namespace, importName, err)
			err = nil
			return
		}
		klog.Errorf("Create or update service %s/%s in client cluster failed, error: %v", namespace, importName, err)
		return
	}

	masterEndpointSlices, err := c.masterEndpointSliceLister.EndpointSlices(namespace).List(labels.SelectorFromSet(map[string]string{
		ServiceKey: importName,
	}))
	if err != nil {
		klog.Errorf("Get service from master cluster failed, error: %v", err)
		return
	}

	for _, eps := range masterEndpointSlices {
		if !helper.HasAnnotation(eps.ObjectMeta, ServiceExportLabelKey) {
			klog.V(4).Infof("ServiceEndpointSlice %s/%s has not been exported in master, ignore it", namespace, eps.Name)
			return
		}
		err = c.importEndpointSliceHandler(eps, serviceImport)
		if err != nil {
			klog.Errorf("Create or update service %s/%s in client cluster failed, error: %v", namespace, importName, err)
			return
		}
	}

	c.clientEventRecorder.Event(serviceImport, v1.EventTypeNormal, "Synced", "serviceImport has been synced successfully")
	klog.V(4).Infof("Handler serviceImport: finished processing %s", importName)
}

func (c *ServiceImportController) shouldEnqueueEndpointSlice(m metav1.ObjectMeta) bool {
	return helper.HasAnnotation(m, ServiceExportLabelKey)
}

func (c *ServiceImportController) cleanupServiceAndEndpointSlice(namespace, name string) error {
	service, err := c.clientServiceLister.Services(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("ServiceImport %s/%s is deleting and Service %s/%s is not found, ignore it", namespace, name, namespace, name)
			return nil
		}
		klog.Errorf("ServiceImport %s/%s is deleting but clean up service failed, Error: %v", namespace, name, err)
		return err
	}

	if !helper.HasAnnotation(service.ObjectMeta, ServiceImportLabelKey) {
		klog.V(4).Infof("Service %s/%s is not managed by kosmos, ignore it", namespace, name)
		return nil
	}

	err = c.client.CoreV1().Services(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("ServiceImport %s/%s is deleting and Service %s/%s is not found, ignore it", namespace, name, namespace, name)
			return nil
		}
		klog.Errorf("ServiceImport %s/%s is deleting but clean up service failed, Error: %v", namespace, name, err)
		return err
	}

	labelSelector := fmt.Sprintf("%s=%s", ServiceKey, name)
	err = c.client.DiscoveryV1().EndpointSlices(namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		klog.Errorf("ServiceImport %s/%s is deleting but clean up endpointSlices failed, Error: %v", namespace, name, err)
		return err
	}
	return nil
}

func (c *ServiceImportController) importServiceHandler(masterService *v1.Service, serviceImport *mcsv1alpha1.ServiceImport) error {
	clientService := generateService(masterService, serviceImport)
	err := c.createOrUpdateServiceInClient(clientService)
	if err != nil {
		return err
	}
	return nil
}

func (c *ServiceImportController) createOrUpdateServiceInClient(service *v1.Service) error {
	oldService, err := c.clientServiceLister.Services(service.Namespace).Get(service.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := c.client.CoreV1().Services(service.Namespace).Create(context.TODO(), service, metav1.CreateOptions{})
			if err != nil {
				klog.Errorf("Create serviceImport service(%s/%s) in client failed, Error: %v", service.Namespace, service.Name, err)
				return err
			} else {
				return nil
			}
		}
		return err
	}

	retainServiceFields(oldService, service)

	_, err = c.client.CoreV1().Services(service.Namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Update serviceImport service(%s/%s) in client failed, Error: %v", service.Namespace, service.Name, err)
		return err
	}
	return nil
}

func (c *ServiceImportController) importEndpointSliceHandler(endpointSlice *discoveryv1.EndpointSlice, serviceImport *mcsv1alpha1.ServiceImport) error {
	newEndpointSlice := endpointSlice.DeepCopy()
	if metav1.HasAnnotation(serviceImport.ObjectMeta, ConnectedEndpointsKey) || metav1.HasAnnotation(serviceImport.ObjectMeta, DisconnectedEndpointsKey) {
		annotationValue := helper.GetLabelOrAnnotationValue(serviceImport.Annotations, DisconnectedEndpointsKey)
		disConnectedAddress := strings.Split(annotationValue, ",")
		clearEndpointSlice(newEndpointSlice, disConnectedAddress)
	}

	return c.createOrUpdateEndpointSliceInClient(newEndpointSlice, serviceImport.Name)
}

func clearEndpointSlice(slice *discoveryv1.EndpointSlice, disconnectedAddress []string) {
	disconnectedAddressMap := make(map[string]struct{})
	for _, name := range disconnectedAddress {
		disconnectedAddressMap[name] = struct{}{}
	}

	endpoints := slice.Endpoints
	for i := range endpoints {
		newAddresses := make([]string, 0)
		for _, address := range endpoints[i].Addresses {
			if _, found := disconnectedAddressMap[address]; !found {
				newAddresses = append(newAddresses, address)
			}
		}
		endpoints[i].Addresses = newAddresses
	}
}

func (c *ServiceImportController) createOrUpdateEndpointSliceInClient(endpointSlice *discoveryv1.EndpointSlice, serviceName string) error {
	newSlice := retainEndpointSlice(endpointSlice, serviceName)
	_, err := c.client.DiscoveryV1().EndpointSlices(newSlice.Namespace).Create(context.TODO(), newSlice, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			err = c.updateEndpointSlice(newSlice, c.client)
			if err != nil {
				klog.Errorf("Update endpointSlice(%s/%s) in client failed, Error: %v", newSlice.Namespace, newSlice.Name, err)
				return err
			}
			return nil
		}
		klog.Errorf("Create endpointSlice(%s/%s) in client failed, Error: %v", newSlice.Namespace, newSlice.Name, err)
		return err
	}
	return nil
}

func retainEndpointSlice(original *discoveryv1.EndpointSlice, serviceName string) *discoveryv1.EndpointSlice {
	endpointSlice := original.DeepCopy()
	endpointSlice.ObjectMeta = metav1.ObjectMeta{
		Namespace: original.Namespace,
		Name:      original.Name,
	}
	helper.AddEndpointSliceAnnotation(endpointSlice, ServiceImportLabelKey, MCSLabelValue)
	helper.AddEndpointSliceLabel(endpointSlice, ServiceKey, serviceName)
	return endpointSlice
}

func (c *ServiceImportController) cleanupEndpointSliceInClient(namespace, sliceName string) error {
	service, err := c.clientEndpointSliceLister.EndpointSlices(namespace).Get(sliceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("EndpointSlice %s/%s is deleting but not found, ignore it", namespace, sliceName)
			return nil
		}
		klog.Errorf("EndpointSlice %s/%s is deleting but get it from client failed, Error: %v", namespace, sliceName, err)
		return err
	}

	if !helper.HasAnnotation(service.ObjectMeta, ServiceImportLabelKey) {
		klog.V(4).Infof("EndpointSlice %s/%s is not managed by kosmos, ignore it", namespace, sliceName)
		return nil
	}

	err = c.client.CoreV1().Endpoints(namespace).Delete(context.TODO(), sliceName, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("EndpointSlice %s/%s is deleting and endpointSlice %s/%s is not found, ignore it", namespace, sliceName)
			return nil
		}
		klog.Errorf("EndpointSlice %s/%s is deleting but clean up endpointSlice failed, Error: %v", namespace, sliceName, err)
		return err
	}
	return nil
}

func generateService(service *v1.Service, serviceImport *mcsv1alpha1.ServiceImport) *v1.Service {
	clusterIP := v1.ClusterIPNone
	if podutils.IsServiceIPSet(service) {
		clusterIP = ""
	}

	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: serviceImport.Namespace,
			Name:      service.Name,
			Annotations: map[string]string{
				ServiceImportLabelKey: MCSLabelValue,
			},
		},
		Spec: v1.ServiceSpec{
			Type:           v1.ServiceTypeClusterIP,
			ClusterIP:      clusterIP,
			Ports:          servicePorts(serviceImport),
			IPFamilies:     service.Spec.IPFamilies,
			IPFamilyPolicy: service.Spec.IPFamilyPolicy,
		},
	}
}

func servicePorts(serviceImport *mcsv1alpha1.ServiceImport) []v1.ServicePort {
	ports := make([]v1.ServicePort, len(serviceImport.Spec.Ports))
	for i, p := range serviceImport.Spec.Ports {
		ports[i] = v1.ServicePort{
			Name:        p.Name,
			Protocol:    p.Protocol,
			Port:        p.Port,
			AppProtocol: p.AppProtocol,
		}
	}
	return ports
}

func retainServiceFields(oldSvc, newSvc *v1.Service) {
	newSvc.Spec.ClusterIP = oldSvc.Spec.ClusterIP
	newSvc.ResourceVersion = oldSvc.ResourceVersion
}
