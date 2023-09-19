package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/knode-manager/adapters"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/utils"
)

type PVCController struct {
	factor  string
	handler adapters.PVCHandler

	client          kubernetes.Interface
	lister          lister.PersistentVolumeClaimLister
	synced          cache.InformerSynced
	upstreamQueue   workqueue.RateLimitingInterface
	downstreamQueue workqueue.RateLimitingInterface
}

func NewPVCController(adapter adapters.PVCHandler, client kubernetes.Interface, factor string) (*PVCController, error) {
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 30*time.Second)
	pvc := &PVCController{
		handler:         adapter,
		client:          client,
		factor:          factor,
		upstreamQueue:   workqueue.NewNamedRateLimitingQueue(rateLimiter, "kNode pvc-controller"),
		downstreamQueue: workqueue.NewNamedRateLimitingQueue(rateLimiter, "kNode pvc-controller"),
	}

	mInformer := kubeinformers.NewSharedInformerFactory(client, 0).Core().V1().PersistentVolumeClaims()
	_, _ = mInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: pvc.updatePVC,
		DeleteFunc: pvc.deletePVC,
	})
	pvc.lister = mInformer.Lister()
	pvc.synced = mInformer.Informer().HasSynced

	return pvc, nil
}

func (p *PVCController) updatePVC(oldObj, newObj interface{}) {
	newPVC := newObj.(*corev1.PersistentVolumeClaim)
	if newPVC.Namespace == metav1.NamespaceSystem || newPVC.Name == "kubernetes" {
		key, err := cache.MetaNamespaceKeyFunc(newObj)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		p.upstreamQueue.Add(key)
		klog.V(6).Info("kosmos update pvc, enqueue: %v", key)
	} else {
		klog.V(6).Infof("kosmos ignore pvc change, pv: ", newPVC.Name)
	}
}

func (p *PVCController) deletePVC(obj interface{}) {
	pvc := obj.(*corev1.PersistentVolumeClaim)
	if pvc.Namespace == metav1.NamespaceSystem || pvc.Name == "kubernetes" {
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		p.upstreamQueue.Add(key)
		klog.V(6).Info("kosmos delete pvc, enqueue: %v", key)
	} else {
		klog.V(6).Infof("kosmos ignore pvc change, pv: ", pvc.Name)
	}
}

func (p *PVCController) Run(ctx context.Context) error {
	p.handler.Notify(ctx, func(pvc *corev1.PersistentVolumeClaim) {
		p.downstreamQueue.Add(pvc)
	})
	defer p.upstreamQueue.ShutDown()
	defer p.downstreamQueue.ShutDown()
	klog.Infof("kosmos starting pvc controller")
	defer klog.Infof("kosmos shutting pvc controller")
	if !cache.WaitForCacheSync(ctx.Done(), p.synced) {
		return fmt.Errorf("kosmos cannot sync pvc cache")
	}
	klog.Infof("kosmos sync pvc cache success")

	go wait.Until(p.syncDownstream, 0, ctx.Done())
	go wait.Until(p.syncUpstream, 0, ctx.Done())

	ctx.Done()
	return nil
}

func (p *PVCController) syncUpstream() {
	pvc2key, result := p.upstreamQueue.Get()
	if result {
		klog.Errorf("kosmos sync upstream: get pvc2key from upstream queue failed")
		return
	}
	defer p.upstreamQueue.Done(pvc2key)
	ns, pvcName, err := cache.SplitMetaNamespaceKey(pvc2key.(string))
	if err != nil {
		p.upstreamQueue.Forget(pvc2key.(string))
		return
	}
	klog.Infof("kosmos sync upstream: start sync upstream pvc, pvc: %v", pvc2key.(string))
	pvcLifecycle := false
	pvcInstance, err := p.lister.PersistentVolumeClaims(ns).Get(pvcName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("kosmos sync upstream: get pvc from list failed, error: %v", err)
			return
		}
		pvcLifecycle = true
	}
	defer func() {
		if err != nil {
			p.upstreamQueue.AddRateLimited(pvc2key)
			return
		}
		p.upstreamQueue.Forget(pvc2key)
	}()

	if pvcLifecycle || pvcInstance.DeletionTimestamp != nil {
		err = p.handler.Delete(context.TODO(), pvcInstance)
		if err != nil {
			klog.Errorf("kosmos sync upstream: delete pvc from downstream list failed, pv: %v", pvcInstance.Name)
			return
		}
		klog.Infof("kosmos sync upstream: delete pvc from downstream list succeed, pv: %v", pvcInstance.Name)
	}
}

func (p *PVCController) syncDownstream() {
	pvc2key, result := p.downstreamQueue.Get()
	if result {
		klog.Errorf("kosmos sync downstream: get pvc2key from downstream queue failed")
		return
	}
	defer p.downstreamQueue.Done(pvc2key)
	pvcNs, pvcName, err := cache.SplitMetaNamespaceKey(pvc2key.(string))
	if err != nil {
		p.downstreamQueue.Forget(pvc2key.(string))
		return
	}
	klog.Infof("kosmos sync downstream: start sync downstream pvc, pvc: %v", pvc2key.(string))
	pvcInstance, err := p.handler.Get(context.TODO(), pvcNs, pvcName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("kosmos sync downstream: get pvc from downstream list failed, err: %v", err)
			return
		}
		klog.Warningf("kosmos sync downstream: pvc has been deleted from downstream list, pvc: %v", pvcName)
	}

	klog.Infof("kosmos sync downstream: start sync pvc status, pv: %v", pvcName)
	key2pvc, err := cache.MetaNamespaceKeyFunc(pvcInstance)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	defer func() {
		if err != nil {
			klog.Error(err)
			p.downstreamQueue.AddRateLimited(key2pvc)
			return
		}
	}()
	pvcInstanceDeepCopy := pvcInstance.DeepCopy()
	upstreamPVC, err := p.lister.PersistentVolumeClaims(pvcInstance.Namespace).Get(pvcInstance.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("kosmos sync downstream: get pvc from upstream list failed, pv: %v", err)
			return
		}
		klog.Warningf("kosmos sync downstream: pvc has been deleted from upstream list, pvc: %v", pvcName)
	}
	if err = p.filterPVC(pvcInstanceDeepCopy, p.factor); err != nil {
		return
	}
	_, err = p.patchPVC(upstreamPVC, pvcInstanceDeepCopy, p.client)
	if err != nil {
		klog.Errorf("kosmos sync downstream: patch pvc from upstream failed, pv: %v", pvcInstance.Name)
		return
	}
	p.downstreamQueue.Forget(key2pvc)
	klog.Infof("kosmos sync downstream: finish sync downstream pvc, pvc: %v", pvc2key.(string))
}

func (p *PVCController) patchPVC(old, new *corev1.PersistentVolumeClaim, c kubernetes.Interface) (*corev1.PersistentVolumeClaim, error) {
	if reflect.DeepEqual(old.Spec, new.Spec) && reflect.DeepEqual(old.Status, new.Status) {
		return old, nil
	}

	oldByte, err := json.Marshal(old)
	if err != nil {
		return nil, err
	}
	newByte, err := json.Marshal(new)
	if err != nil {
		return nil, err
	}
	patch, err := jsonpatch.CreateMergePatch(oldByte, newByte)
	if err != nil {
		return nil, err
	}

	newPVC, err := c.CoreV1().PersistentVolumeClaims(old.Namespace).Patch(context.TODO(), old.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return old, err
	}
	return newPVC, nil
}

func (p *PVCController) filterPVC(pvc *corev1.PersistentVolumeClaim, factor string) error {
	labelSelector := pvc.Spec.Selector.DeepCopy()
	pvc.Spec.Selector = nil
	utils.TrimObjectMeta(&pvc.ObjectMeta)
	if pvc.Annotations == nil {
		pvc.Annotations = map[string]string{}
	}
	pvc.Annotations["kosmos.io/global"] = "true"
	if labelSelector != nil {
		labelStr, err := json.Marshal(labelSelector)
		if err != nil {
			return err
		}
		pvc.Annotations["labelSelector"] = string(labelStr)
	}
	if len(pvc.Annotations["volume.kubernetes.io/selected-node"]) != 0 {
		pvc.Annotations["volume.kubernetes.io/selected-node"] = factor
	}
	return nil
}
