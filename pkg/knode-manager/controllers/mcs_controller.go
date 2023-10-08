package controllers

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
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	discoveryv1listers "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/utils/podutils"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	kosmosinformer "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	kosmoslisters "github.com/kosmos.io/kosmos/pkg/generated/listers/apis/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils/helper"
)

const (
	ServiceKey                   = "kubernetes.io/service-name"
	MCSControllerName            = "mcs-controller"
	ServiceExportLabelKey        = "kosmos.io/service-export"
	ServiceImportLabelKey        = "kosmos.io/service-import"
	MCSLabelValue                = "ture"
	ConnectedEndpointSliceKey    = "kosmos.io/connected-endpointslices"
	DisconnectedEndpointSliceKey = "kosmos.io/disconnected-endpointslices"
)

type MCSController struct {
	master                            kubernetes.Interface
	kosmosMaster                      kosmosversioned.Interface
	masterEventRecorder               record.EventRecorder
	masterServiceExportLister         kosmoslisters.ServiceExportLister
	masterServiceExportInformerSynced cache.InformerSynced
	masterServiceLister               corev1listers.ServiceLister
	masterServiceInformerSynced       cache.InformerSynced
	masterEndpointSliceLister         discoveryv1listers.EndpointSliceLister
	masterEndpointSliceInformerSynced cache.InformerSynced
	masterEndpointSliceQueue          workqueue.RateLimitingInterface
	masterServiceExportQueue          workqueue.RateLimitingInterface

	client                            kubernetes.Interface
	kosmosClient                      kosmosversioned.Interface
	clientEventRecorder               record.EventRecorder
	clientServiceImportQueue          workqueue.RateLimitingInterface
	clientEndpointSliceQueue          workqueue.RateLimitingInterface
	clientServiceLister               corev1listers.ServiceLister
	clientServiceInformerSynced       cache.InformerSynced
	clientEndpointSliceLister         discoveryv1listers.EndpointSliceLister
	clientEndpointSliceInformerSynced cache.InformerSynced
	clientServiceImportLister         kosmoslisters.ServiceImportLister
	clientServiceImportInformerSynced cache.InformerSynced
}

func NewServiceImportController(master, client kubernetes.Interface, kosmosMaster, kosmosClient kosmosversioned.Interface, masterInformer, clientInformer informers.SharedInformerFactory, kosmosMasterInformer, kosmosClientInformer kosmosinformer.SharedInformerFactory) (*MCSController, error) {
	c := &MCSController{
		master:                   master,
		kosmosMaster:             kosmosMaster,
		masterEndpointSliceQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		masterServiceExportQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		client:                   client,

		kosmosClient:             kosmosClient,
		clientServiceImportQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		clientEndpointSliceQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	masterBroadcaster := record.NewBroadcaster()
	masterBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: master.CoreV1().Events(v1.NamespaceAll)})
	masterEventRecorder := masterBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: ComponentName})
	c.masterEventRecorder = masterEventRecorder

	clientBroadcaster := record.NewBroadcaster()
	clientBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: client.CoreV1().Events(v1.NamespaceAll)})
	clientEventRecorder := clientBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: ComponentName})
	c.clientEventRecorder = clientEventRecorder

	// master
	masterServiceInformer := masterInformer.Core().V1().Services()
	masterEndpointSliceInformer := masterInformer.Discovery().V1().EndpointSlices()
	masterServiceExportInformer := kosmosMasterInformer.Multicluster().V1alpha1().ServiceExports()

	c.masterServiceLister = masterServiceInformer.Lister()
	c.masterServiceInformerSynced = masterServiceInformer.Informer().HasSynced
	c.masterEndpointSliceLister = masterEndpointSliceInformer.Lister()
	c.masterEndpointSliceInformerSynced = masterEndpointSliceInformer.Informer().HasSynced
	c.masterServiceExportLister = masterServiceExportInformer.Lister()
	c.masterServiceExportInformerSynced = masterServiceInformer.Informer().HasSynced

	_, err := masterServiceExportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.masterServiceExportAdded,
		UpdateFunc: c.masterServiceExportUpdated,
		DeleteFunc: c.masterServiceExportDeleted,
	})
	if err != nil {
		return nil, err
	}
	_, err = masterEndpointSliceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.masterEndpointSliceAdded,
		UpdateFunc: c.masterEndpointSliceUpdated,
		DeleteFunc: c.masterEndpointSliceDeleted,
	})
	if err != nil {
		return nil, err
	}

	// client
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

func (c *MCSController) Run(ctx context.Context) error {
	defer c.masterServiceExportQueue.ShutDown()
	defer c.masterEndpointSliceQueue.ShutDown()
	defer c.clientEndpointSliceQueue.ShutDown()
	defer c.clientServiceImportQueue.ShutDown()

	klog.Infof("Starting %s", MCSControllerName)
	defer klog.Infof("Shutting %s", MCSControllerName)

	stopCh := ctx.Done()
	workers := 1

	if !cache.WaitForCacheSync(stopCh, c.masterServiceExportInformerSynced, c.masterServiceInformerSynced, c.masterEndpointSliceInformerSynced) {
		return fmt.Errorf("cannot sync endpointSlice serviceExport service caches from master")
	}
	klog.Infof("Master endpointSlice serviceExport service caches synced")

	if !cache.WaitForCacheSync(stopCh, c.clientServiceImportInformerSynced, c.clientEndpointSliceInformerSynced, c.clientServiceInformerSynced) {
		return fmt.Errorf("cannot sync endpointSlice serviceImport service caches from client")
	}
	klog.Infof("Client endpointSlice serviceImport service caches synced")

	for i := 0; i < workers; i++ {
		go wait.Until(c.syncMasterEndpointSlice, 0, stopCh)
		go wait.Until(c.syncMasterServiceExport, 0, stopCh)
		go wait.Until(c.syncClientEndpointSlice, 0, stopCh)
		go wait.Until(c.syncClientServiceImport, 0, stopCh)
	}

	<-stopCh
	return nil
}

func (c *MCSController) masterServiceExportAdded(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Master serviceExport add ", "key ", key)
	c.masterServiceExportQueue.Add(key)
}

func (c *MCSController) masterServiceExportUpdated(_, new interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Master serviceExport update ", "key ", key)
	c.masterServiceExportQueue.Add(key)
}

func (c *MCSController) masterServiceExportDeleted(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Master serviceExport deleted ", "key ", key)
	c.masterServiceExportQueue.Add(key)
}

func (c *MCSController) masterEndpointSliceAdded(obj interface{}) {
	eps := obj.(*discoveryv1.EndpointSlice)
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Enqueue endpointSlice in master add ", "key ", key)
	c.masterEndpointSliceQueue.Add(key)

	if c.shouldEnqueueEndpointSlice(eps.ObjectMeta) {
		klog.Info("Enqueue endpointSlice in knode add ", "key ", key)
		c.clientEndpointSliceQueue.Add(key)
	}
}

func (c *MCSController) masterEndpointSliceUpdated(_, new interface{}) {
	eps := new.(*discoveryv1.EndpointSlice)
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	if c.shouldEnqueueEndpointSlice(eps.ObjectMeta) {
		klog.Info("Enqueue endpointSlice in knode add ", "key ", key)
		c.clientEndpointSliceQueue.Add(key)
	}
}

func (c *MCSController) masterEndpointSliceDeleted(obj interface{}) {
	eps := obj.(*discoveryv1.EndpointSlice)
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	if c.shouldEnqueueEndpointSlice(eps.ObjectMeta) {
		klog.Info("Enqueue endpointSlice in knode add ", "key ", key)
		c.clientEndpointSliceQueue.Add(key)
	}
}

func (c *MCSController) clientServiceImportAdded(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Client serviceImport add ", "key ", key)
	c.clientServiceImportQueue.Add(key)
}

func (c *MCSController) clientServiceImportUpdated(_, new interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Client serviceImport update ", "key ", key)
	c.clientServiceImportQueue.Add(key)
}

func (c *MCSController) clientServiceImportDeleted(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Client serviceImport delete ", "key ", key)
	c.clientServiceImportQueue.Add(key)
}

func (c *MCSController) syncMasterEndpointSlice() {
	keyObj, quit := c.masterEndpointSliceQueue.Get()
	if quit {
		return
	}
	defer c.masterEndpointSliceQueue.Done(keyObj)

	key := keyObj.(string)
	namespace, epsName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		err = nil
		return
	}
	klog.V(4).Infof("Started endpointSlice processing %s/%s", namespace, epsName)

	defer func() {
		if err != nil {
			c.masterEndpointSliceQueue.AddRateLimited(keyObj)
		}
		c.masterEndpointSliceQueue.Forget(keyObj)
	}()

	endpointSlice, err := c.masterEndpointSliceLister.EndpointSlices(namespace).Get(epsName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Get endpointSlice %s/%s from master cluster failed, error: %v", namespace, epsName, err)
			return
		}
		err = nil
		klog.V(3).Infof("EndpointSlice %s/%s deleted", namespace, epsName)
		return
	}

	serviceName := helper.GetLabelOrAnnotationValue(endpointSlice.GetLabels(), ServiceKey)
	if serviceName == "" {
		klog.V(4).Infof("EndpointSlice %s/%s has a empty service name label,ignore it", namespace, epsName)
		return
	}

	_, err = c.masterServiceExportLister.ServiceExports(namespace).Get(serviceName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Get serviceExport from master cluster failed, error: %v", err)
			return
		}
		err = nil
		klog.V(4).Infof("EndpointSlice %s/%s has no related serviceExport,ignore it", namespace, epsName)
		return
	}

	if helper.GetLabelOrAnnotationValue(endpointSlice.GetLabels(), ServiceExportLabelKey) != MCSLabelValue {
		helper.AddEndpointSliceLabel(endpointSlice, ServiceExportLabelKey, MCSLabelValue)
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			_, updateErr := c.master.DiscoveryV1().EndpointSlices(namespace).Update(context.TODO(), endpointSlice, metav1.UpdateOptions{})
			if updateErr == nil {
				return nil
			}
			updated, getErr := c.master.DiscoveryV1().EndpointSlices(namespace).Get(context.TODO(), endpointSlice.Name, metav1.GetOptions{})
			if getErr == nil {
				//make a copy, so we don't mutate the shared cache
				endpointSlice = updated.DeepCopy()
			} else {
				klog.Errorf("Failed to get updated endpointSlice %s/%s: %v", namespace, endpointSlice.Name, err)
			}
			return updateErr
		})
		if err != nil {
			klog.Errorf("Update endpointSlice (%s/%s) status failed, Error: %v", namespace, endpointSlice.Name, err)
			return
		}
	}

	c.masterEventRecorder.Event(endpointSlice, v1.EventTypeNormal, "Synced",
		"EndpointSlice has been add a annotation for a existed serviceExport")
	klog.V(4).Infof("Handler EndpointSlice: finished processing %s/%s", namespace, epsName)
}

func (c *MCSController) syncMasterServiceExport() {
	keyObj, quit := c.masterServiceExportQueue.Get()
	if quit {
		return
	}
	defer c.masterServiceExportQueue.Done(keyObj)

	key := keyObj.(string)
	namespace, serviceExportName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		err = nil
		return
	}
	klog.V(4).Infof("Started serviceExport processing %s/%s", namespace, serviceExportName)

	defer func() {
		if err != nil {
			c.masterServiceExportQueue.AddRateLimited(keyObj)
		}
		c.masterServiceExportQueue.Forget(keyObj)
	}()

	shouldRemoveAnnotation := false
	serviceExport, err := c.masterServiceExportLister.ServiceExports(namespace).Get(serviceExportName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Get serviceExport from master cluster failed, error: %v", err)
			return
		}
		err = nil
		shouldRemoveAnnotation = true
		klog.V(3).Infof("ServiceExport  %s/%s deleted", namespace, serviceExportName)
	}

	endpointSlices, err := c.masterEndpointSliceLister.EndpointSlices(namespace).List(labels.SelectorFromSet(map[string]string{
		ServiceKey: serviceExportName,
	}))
	if err != nil {
		klog.Errorf("List endpointSlice in %s failed, Error: %v", namespace, err)
		return
	}

	for _, eps := range endpointSlices {
		if eps.DeletionTimestamp != nil {
			klog.V(4).Infof("EndpointSlice %s/%s is deleting an do not need remove serviceExport annotation", namespace, eps.Name)
			continue
		}
		if serviceExport.DeletionTimestamp != nil || shouldRemoveAnnotation {
			helper.RemoveLabel(eps, ServiceExportLabelKey)
			err := c.updateEndpointSlice(eps, c.master)
			if err != nil {
				klog.Errorf("Update endpointSlice (%s/%s) failed, Error: %v", namespace, eps.Name, err)
				return
			}
		}
	}

	for _, eps := range endpointSlices {
		if eps.DeletionTimestamp != nil {
			klog.V(4).Infof("EndpointSlice %s/%s is deleting an do not need add serviceExport annotation", namespace, eps.Name)
			continue
		}
		helper.AddEndpointSliceLabel(eps, ServiceExportLabelKey, MCSLabelValue)
		err := c.updateEndpointSlice(eps, c.master)
		if err != nil {
			klog.Errorf("Update endpointSlice (%s/%s) failed, Error: %v", namespace, eps.Name, err)
			return
		}
	}

	c.masterEventRecorder.Event(serviceExport, v1.EventTypeNormal, "Synced",
		"serviceExport has been synced to endpointSlice's annotation successfully")
	klog.V(4).Infof("Handler serviceExport: finished processing %s/%s", namespace, serviceExportName)
}

func (c *MCSController) updateEndpointSlice(eps *discoveryv1.EndpointSlice, client kubernetes.Interface) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, updateErr := client.DiscoveryV1().EndpointSlices(eps.Namespace).Update(context.TODO(), eps, metav1.UpdateOptions{})
		if updateErr == nil {
			return nil
		}
		updated, getErr := client.DiscoveryV1().EndpointSlices(eps.Namespace).Get(context.TODO(), eps.Name, metav1.GetOptions{})
		if getErr == nil {
			//make a copy, so we don't mutate the shared cache
			eps = updated.DeepCopy()
		} else {
			klog.Errorf("Failed to get updated endpointSlice %s/%s: %v", eps.Namespace, eps.Name, getErr)
		}
		return updateErr
	})
}

func (c *MCSController) syncClientEndpointSlice() {
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

	shouldCleanup := false
	masterSlice, err := c.masterEndpointSliceLister.EndpointSlices(namespace).Get(sliceName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Get endpointSlice in master cluster failed, error: %v", err)
			return
		}
		err = nil
		shouldCleanup = true
		klog.V(3).Infof("EndpointSlice %/%s deleted", namespace, sliceName)
	}

	if masterSlice.DeletionTimestamp != nil || shouldCleanup {
		err := c.cleanupEndpointSliceInClient(masterSlice)
		if err != nil {
			klog.Errorf("Cleanup endpointSlice in client cluster failed, error: %v", err)
			return
		}
	}

	serviceImportName := helper.GetLabelOrAnnotationValue(masterSlice.Labels, ServiceKey)
	serviceImport, err := c.clientServiceImportLister.ServiceImports(namespace).Get(serviceImportName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Get serviceImport from client cluster failed, error: %v", err)
			return
		}
		err = nil
		klog.V(3).Infof("EndpointSlice %/%s does not have a serviceImport in client,should not synced", namespace, sliceName)
		return
	}

	err = c.importEndpointSliceHandler(masterSlice, serviceImport)
	if err != nil {
		klog.Errorf("Create or update endpointSlice %/%s in client cluster failed, error: %v", namespace, sliceName, err)
		return
	}

	c.clientEventRecorder.Event(masterSlice, v1.EventTypeNormal, "Synced",
		"endpointSlice has been synced successfully")
	klog.V(4).Infof("Handler endpointSlice: finished processing %s/%s", namespace, sliceName)
}

func (c *MCSController) syncClientServiceImport() {
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
		klog.V(3).Infof("ServiceImport %/%s deleted", namespace, importName)
	}

	if serviceImport.DeletionTimestamp != nil || shouldCleanup {
		err := c.cleanupServiceAndEndpointSlice(serviceImport)
		if err != nil {
			klog.Errorf("Cleanup  service and endpointSlice in client cluster failed, error: %v", err)
			return
		}
	}

	masterService, err := c.masterServiceLister.Services(namespace).Get(importName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Get service from master cluster failed, error: %v", err)
			return
		}
		err = nil
		klog.V(3).Infof("Service %/%s deleted in master", namespace, importName)
		return
	}

	err = c.importServiceHandler(masterService, serviceImport)
	if err != nil {
		klog.Errorf("Create or update service %/%s in client cluster failed, error: %v", namespace, importName, err)
		return
	}

	masterEndpointSlices, err := c.clientEndpointSliceLister.EndpointSlices(namespace).List(labels.SelectorFromSet(map[string]string{
		ServiceKey: importName,
	}))
	if err != nil {
		klog.Errorf("Get service from master cluster failed, error: %v", err)
		return
	}

	for _, eps := range masterEndpointSlices {
		if !helper.HasLabel(eps.ObjectMeta, ServiceExportLabelKey) {
			klog.V(4).Infof("ServiceEndpointSlice %/%s has not beend exported in master,ignore it ", namespace, eps.Name)
			return
		}
		err = c.importEndpointSliceHandler(eps, serviceImport)
		if err != nil {
			klog.Errorf("Create or update service %/%s in client cluster failed, error: %v", namespace, importName, err)
			return
		}
	}

	c.clientEventRecorder.Event(serviceImport, v1.EventTypeNormal, "Synced",
		"serviceImport has been synced successfully")
	klog.V(4).Infof("Handler serviceImport: finished processing %s", importName)
}

func (c *MCSController) shouldEnqueueEndpointSlice(m metav1.ObjectMeta) bool {
	return helper.HasLabel(m, ServiceExportLabelKey)
}

func (c *MCSController) cleanupServiceAndEndpointSlice(serviceImport *mcsv1alpha1.ServiceImport) error {
	namespace := serviceImport.Namespace
	serviceName := serviceImport.Name

	service, err := c.clientServiceLister.Services(serviceImport.Namespace).Get(serviceImport.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("ServiceImport %s/%s is deleting and Service %s/%s is not found, ignore it", namespace, serviceName, namespace, serviceName)
			return nil
		}
		klog.Errorf("ServiceImport %s/%s is deleting but clean up service failed, Error: %v", namespace, serviceName, err)
		return err
	}

	if !helper.HasLabel(service.ObjectMeta, ServiceImportLabelKey) {
		klog.V(4).Infof("Service %s/%s is not managed by kosmos, ignore it", namespace, serviceName)
		return nil
	}

	err = c.client.CoreV1().Services(namespace).Delete(context.TODO(), serviceName, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("ServiceImport %s/%s is deleting and Service %s/%s is not found, ignore it", namespace, serviceName, namespace, serviceName)
			return nil
		}
		klog.Errorf("ServiceImport %s/%s is deleting but clean up service failed, Error: %v", namespace, serviceName, err)
		return err
	}

	err = c.client.DiscoveryV1().EndpointSlices(namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", ServiceKey, serviceName),
	})
	if err != nil {
		klog.Errorf("ServiceImport %s/%s is deleting but clean up endpointSlices failed, Error: %v", namespace, serviceName, err)
		return err
	}
	return nil
}

func (c *MCSController) importServiceHandler(masterService *v1.Service, serviceImport *mcsv1alpha1.ServiceImport) error {
	clientService := generateService(masterService, serviceImport)
	err := c.createOrUpdateServiceInClient(clientService)
	if err != nil {
		return err
	}
	return nil
}

func (c *MCSController) createOrUpdateServiceInClient(service *v1.Service) error {
	oldService, err := c.clientServiceLister.Services(service.Namespace).Get(service.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, err := c.client.CoreV1().Services(service.Namespace).Create(context.TODO(), service, metav1.CreateOptions{}); err != nil {
				klog.Errorf("Create serviceImport service(%s/%s) in client failed, Error: %v", service.Namespace, service.Name, err)
				return err
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

func (c *MCSController) importEndpointSliceHandler(eps *discoveryv1.EndpointSlice, serviceImport *mcsv1alpha1.ServiceImport) error {
	if metav1.HasAnnotation(serviceImport.ObjectMeta, ConnectedEndpointSliceKey) || metav1.HasAnnotation(serviceImport.ObjectMeta, DisconnectedEndpointSliceKey) {
		annotationValue := helper.GetLabelOrAnnotationValue(serviceImport.Annotations, ConnectedEndpointSliceKey)
		connectedSlices := strings.Split(annotationValue, ",")
		if !stringInSlice(eps.Name, connectedSlices) {
			return nil
		}
	}
	return c.createOrUpdateEndpointSliceInClient(eps)
}

func (c *MCSController) createOrUpdateEndpointSliceInClient(endpointSlice *discoveryv1.EndpointSlice) error {
	helper.AddEndpointSliceLabel(endpointSlice, ServiceImportLabelKey, MCSLabelValue)

	_, err := c.client.DiscoveryV1().EndpointSlices(endpointSlice.Namespace).Create(context.TODO(), endpointSlice, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			err = c.updateEndpointSlice(endpointSlice, c.client)
			if err != nil {
				return err
			}
			return nil
		}
		klog.Errorf("Create endpointSlice(%s/%s) in client failed, Error: %v", endpointSlice.Namespace, endpointSlice.Name, err)
		return err
	}
	return nil
}

func (c *MCSController) cleanupEndpointSliceInClient(slice *discoveryv1.EndpointSlice) interface{} {
	namespace := slice.Namespace
	sliceName := slice.Name

	service, err := c.clientEndpointSliceLister.EndpointSlices(namespace).Get(namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("EndpointSlice %s/%s is deleting but not found, ignore it", namespace, sliceName, namespace, sliceName)
			return nil
		}
		klog.Errorf("EndpointSlice %s/%s is deleting but get it from client failed, Error: %v", namespace, sliceName, err)
		return err
	}

	if !helper.HasLabel(service.ObjectMeta, ServiceImportLabelKey) {
		klog.V(4).Infof("EndpointSlice %s/%s is not managed by kosmos, ignore it", namespace, sliceName)
		return nil
	}

	err = c.client.CoreV1().Endpoints(namespace).Delete(context.TODO(), sliceName, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("EndpointSlice %s/%s is deleting and endpointSlice %s/%s is not found, ignore it", namespace, sliceName, namespace, sliceName)
			return nil
		}
		klog.Errorf("EndpointSlice %s/%s is deleting but clean up endpointSlice failed, Error: %v", namespace, sliceName, err)
		return err
	}
	return nil
}

func stringInSlice(s string, slice []string) bool {
	for _, element := range slice {
		if element == s {
			return true
		}
	}
	return false
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
			Labels: map[string]string{
				ServiceImportLabelKey: MCSLabelValue,
			},
		},
		Spec: v1.ServiceSpec{
			Type:      v1.ServiceTypeClusterIP,
			ClusterIP: clusterIP,
			Ports:     servicePorts(serviceImport),
		},
	}
}

func servicePorts(svcImport *mcsv1alpha1.ServiceImport) []v1.ServicePort {
	ports := make([]v1.ServicePort, len(svcImport.Spec.Ports))
	for i, p := range svcImport.Spec.Ports {
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
