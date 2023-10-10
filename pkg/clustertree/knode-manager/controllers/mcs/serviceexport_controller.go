package mcs

import (
	"context"
	"fmt"

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

	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/controllers"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	kosmosinformer "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	kosmoslisters "github.com/kosmos.io/kosmos/pkg/generated/listers/apis/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/utils/helper"
)

const (
	ServiceExportControllerName = "serviceexport-controller"
)

type ServiceExportController struct {
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
}

func NewServiceExportController(master kubernetes.Interface, kosmosMaster kosmosversioned.Interface, masterInformer informers.SharedInformerFactory, kosmosMasterInformer kosmosinformer.SharedInformerFactory) (*ServiceExportController, error) {
	c := &ServiceExportController{
		master:                   master,
		kosmosMaster:             kosmosMaster,
		masterEndpointSliceQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		masterServiceExportQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	masterBroadcaster := record.NewBroadcaster()
	masterBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: master.CoreV1().Events(v1.NamespaceAll)})
	masterEventRecorder := masterBroadcaster.NewRecorder(scheme.NewSchema(), v1.EventSource{Component: controllers.ComponentName})
	c.masterEventRecorder = masterEventRecorder

	// master
	masterServiceInformer := masterInformer.Core().V1().Services()
	masterEndpointSliceInformer := masterInformer.Discovery().V1().EndpointSlices()
	masterServiceExportInformer := kosmosMasterInformer.Multicluster().V1alpha1().ServiceExports()

	c.masterServiceLister = masterServiceInformer.Lister()
	c.masterServiceInformerSynced = masterServiceInformer.Informer().HasSynced
	c.masterEndpointSliceLister = masterEndpointSliceInformer.Lister()
	c.masterEndpointSliceInformerSynced = masterEndpointSliceInformer.Informer().HasSynced
	c.masterServiceExportLister = masterServiceExportInformer.Lister()
	c.masterServiceExportInformerSynced = masterServiceExportInformer.Informer().HasSynced

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
		//DeleteFunc: c.masterEndpointSliceDeleted,
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *ServiceExportController) Run(stopCh <-chan struct{}) error {
	defer c.masterServiceExportQueue.ShutDown()
	defer c.masterEndpointSliceQueue.ShutDown()

	klog.Infof("Starting %s", ServiceExportControllerName)
	defer klog.Infof("Shutting %s", ServiceExportControllerName)

	workers := 1

	if !cache.WaitForCacheSync(stopCh, c.masterServiceExportInformerSynced, c.masterServiceInformerSynced, c.masterEndpointSliceInformerSynced) {
		return fmt.Errorf("cannot sync endpointSlice serviceExport service caches from master")
	}
	klog.Infof("Master endpointSlice serviceExport service caches synced")

	for i := 0; i < workers; i++ {
		go wait.Until(c.syncMasterEndpointSlice, 0, stopCh)
		go wait.Until(c.syncMasterServiceExport, 0, stopCh)
	}

	<-stopCh
	return nil
}

func (c *ServiceExportController) masterServiceExportAdded(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Master serviceExport add ", "key ", key)
	c.masterServiceExportQueue.Add(key)
}

func (c *ServiceExportController) masterServiceExportUpdated(old, new interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Master serviceExport update ", "key ", key)
	c.masterServiceExportQueue.Add(key)
}

func (c *ServiceExportController) masterServiceExportDeleted(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Master serviceExport deleted ", "key ", key)
	c.masterServiceExportQueue.Add(key)
}

func (c *ServiceExportController) masterEndpointSliceAdded(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Enqueue endpointSlice in master add ", "key ", key)
	c.masterEndpointSliceQueue.Add(key)
}

func (c *ServiceExportController) syncMasterEndpointSlice() {
	keyObj, quit := c.masterEndpointSliceQueue.Get()
	if quit {
		return
	}

	defer c.masterEndpointSliceQueue.Done(keyObj)

	epsKey := keyObj.(string)
	namespace, epsName, err := cache.SplitMetaNamespaceKey(epsKey)
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
		klog.V(4).Infof("EndpointSlice %s/%s has an empty service name label, ignore it", namespace, epsName)
		return
	}

	_, err = c.masterServiceExportLister.ServiceExports(namespace).Get(serviceName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Get serviceExport from master cluster failed, error: %v", err)
			return
		}
		err = nil
		klog.V(4).Infof("EndpointSlice %s/%s has no related serviceExport, ignore it", namespace, epsName)
		return
	}

	if helper.GetLabelOrAnnotationValue(endpointSlice.GetAnnotations(), ServiceExportLabelKey) != MCSLabelValue {
		helper.AddEndpointSliceAnnotation(endpointSlice, ServiceExportLabelKey, MCSLabelValue)
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			_, updateErr := c.master.DiscoveryV1().EndpointSlices(namespace).Update(context.TODO(), endpointSlice, metav1.UpdateOptions{})
			if updateErr == nil {
				return nil
			}
			updated, getErr := c.master.DiscoveryV1().EndpointSlices(namespace).Get(context.TODO(), endpointSlice.Name, metav1.GetOptions{})
			if getErr == nil {
				// Make a copy, so we don't mutate the shared cache
				endpointSlice = updated.DeepCopy()
				helper.AddEndpointSliceAnnotation(endpointSlice, ServiceExportLabelKey, MCSLabelValue)
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

	c.masterEventRecorder.Event(endpointSlice, v1.EventTypeNormal, "Synced", "EndpointSlice has been added an annotation for an existing serviceExport")
	klog.V(4).Infof("Handler EndpointSlice: finished processing %s/%s", namespace, epsName)
}

func (c *ServiceExportController) syncMasterServiceExport() {
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
		klog.V(3).Infof("ServiceExport %s/%s deleted", namespace, serviceExportName)
	}

	endpointSlices, err := c.masterEndpointSliceLister.EndpointSlices(namespace).List(labels.SelectorFromSet(map[string]string{
		ServiceKey: serviceExportName,
	}))
	if err != nil {
		klog.Errorf("List endpointSlice in %s failed, Error: %v", namespace, err)
		return
	}

	if shouldRemoveAnnotation || serviceExport.DeletionTimestamp != nil {
		for _, eps := range endpointSlices {
			if eps.DeletionTimestamp != nil {
				klog.V(4).Infof("EndpointSlice %s/%s is deleting and does not need to remove serviceExport annotation", namespace, eps.Name)
				continue
			}
			helper.RemoveAnnotation(eps, ServiceExportLabelKey)
			err := c.updateEndpointSlice(eps, c.master)
			if err != nil {
				klog.Errorf("Update endpointSlice (%s/%s) failed, Error: %v", namespace, eps.Name, err)
				return
			}
		}
		klog.Infof("ServiceImport (%s/%s) deleted", namespace, serviceExportName)
		return
	}

	for _, eps := range endpointSlices {
		if eps.DeletionTimestamp != nil {
			klog.V(4).Infof("EndpointSlice %s/%s is deleting and does not need to add serviceExport annotation", namespace, eps.Name)
			continue
		}
		helper.AddEndpointSliceAnnotation(eps, ServiceExportLabelKey, MCSLabelValue)
		err := c.updateEndpointSlice(eps, c.master)
		if err != nil {
			klog.Errorf("Update endpointSlice (%s/%s) failed, Error: %v", namespace, eps.Name, err)
			return
		}
	}

	c.masterEventRecorder.Event(serviceExport, v1.EventTypeNormal, "Synced", "serviceExport has been synced to endpointSlice's annotation successfully")
	klog.V(4).Infof("Handler serviceExport: finished processing %s/%s", namespace, serviceExportName)
}

func (c *ServiceExportController) updateEndpointSlice(endpointSlice *discoveryv1.EndpointSlice, client kubernetes.Interface) error {
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

func (c *ServiceExportController) masterEndpointSliceUpdated(old interface{}, new interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Info("Enqueue endpointSlice in master add ", "key ", key)
	c.masterEndpointSliceQueue.Add(key)
}
