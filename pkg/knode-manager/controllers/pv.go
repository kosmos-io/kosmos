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

type PVController struct {
	factor  string
	handler adapters.PVHandler

	client          kubernetes.Interface
	lister          lister.PersistentVolumeLister
	synced          cache.InformerSynced
	upstreamQueue   workqueue.RateLimitingInterface
	downstreamQueue workqueue.RateLimitingInterface
}

func NewPVController(adapter adapters.PVHandler, client kubernetes.Interface, factor string) (*PVController, error) {
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 30*time.Second)
	pv := &PVController{
		handler:         adapter,
		client:          client,
		factor:          factor,
		upstreamQueue:   workqueue.NewNamedRateLimitingQueue(rateLimiter, "asyncPVFromKube"),
		downstreamQueue: workqueue.NewNamedRateLimitingQueue(rateLimiter, "asyncPVFromKube"),
	}

	mInformer := kubeinformers.NewSharedInformerFactory(client, 0).Core().V1().PersistentVolumes()
	_, _ = mInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: pv.deletePV,
	})
	pv.lister = mInformer.Lister()
	pv.synced = mInformer.Informer().HasSynced

	return pv, nil
}

func (p *PVController) Run(ctx context.Context) error {
	p.handler.Notify(ctx, func(pv *corev1.PersistentVolume) {
		p.downstreamQueue.Add(pv)
	})
	defer p.upstreamQueue.ShutDown()
	defer p.downstreamQueue.ShutDown()
	klog.Infof("kosmos starting pv controller")
	defer klog.Infof("kosmos shutting pv controller")
	if !cache.WaitForCacheSync(ctx.Done(), p.synced) {
		return fmt.Errorf("kosmos cannot sync pv cache")
	}
	klog.Infof("kosmos sync pv cache success")

	go wait.Until(p.syncDownstream, 0, ctx.Done())
	go wait.Until(p.syncUpstream, 0, ctx.Done())

	<-ctx.Done()
	return nil
}

func (p *PVController) syncUpstream() {
	pv2key, result := p.upstreamQueue.Get()
	if result {
		klog.Errorf("kosmos sync upstream: get pv2key from upstream queue failed")
		return
	}
	defer p.upstreamQueue.Done(pv2key)
	klog.Infof("kosmos sync upstream: start sync upstream pv, pv: %v", pv2key.(string))
	pvLifecycle := false
	pvInstance, err := p.lister.Get(pv2key.(string))
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("kosmos sync upstream: get pv from list failed, error: %v", err)
			return
		}
		_, err = p.client.CoreV1().PersistentVolumes().Get(context.TODO(), pvInstance.Name, metav1.GetOptions{})
		if !apierrors.IsNotFound(err) {
			klog.Errorf("kosmos sync upstream: get pv from list failed, error: %v", err)
			return
		}
		pvLifecycle = true
	}
	defer func() {
		if err != nil {
			p.upstreamQueue.AddRateLimited(pv2key)
			return
		}
		p.upstreamQueue.Forget(pv2key)
	}()

	if pvLifecycle || pvInstance.DeletionTimestamp != nil {
		err = p.handler.Delete(context.TODO(), pvInstance)
		if err != nil {
			klog.Errorf("kosmos sync upstream: delete pv from downstream list failed, pv: %v", pvInstance.Name)
			return
		}
		klog.Infof("kosmos sync upstream: delete pv from downstream list succeed, pv: %v", pvInstance.Name)
	}
}

func (p *PVController) syncDownstream() {
	pv2key, result := p.downstreamQueue.Get()
	if result {
		klog.Errorf("kosmos sync downstream: get pv2key from downstream queue failed")
		return
	}
	defer p.downstreamQueue.Done(pv2key)
	klog.Infof("kosmos sync downstream: start sync downstream pv, pv: %v", pv2key.(string))
	pvLifecycle := false
	pvInstance, err := p.handler.Get(context.TODO(), pv2key.(string))
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("kosmos sync downstream: get pv from downstream failed, error: %v", err)
			return
		}
		pvLifecycle = true
	}
	defer func() {
		if err != nil {
			p.downstreamQueue.AddRateLimited(pv2key)
			return
		}
		p.downstreamQueue.Forget(pv2key)
	}()

	if pvLifecycle || pvInstance.DeletionTimestamp != nil {
		err = p.client.CoreV1().PersistentVolumes().Delete(context.TODO(), pvInstance.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("kosmos sync downstream: delete pv from upstream failed, pv: %v", pvInstance.Name)
			return
		}
		klog.Infof("kosmos sync downstream: delete pv from upstream succeed, pv: %v", pvInstance.Name)
	}

	klog.Infof("kosmos sync downstream: start sync pv status, pv: %v", pvInstance.Name)
	key2pv, err := cache.MetaNamespaceKeyFunc(pvInstance)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	defer func() {
		if err != nil {
			klog.Error(err)
			p.downstreamQueue.AddRateLimited(key2pv)
			return
		}
	}()
	pvInstanceDeepCopy := pvInstance.DeepCopy()
	upstreamPV, err := p.lister.Get(pvInstance.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("kosmos sync downstream: get pv from upstream list failed, pv: %v", err)
			return
		}
		upstreamPV = pvInstanceDeepCopy
		if err = p.filterPV(upstreamPV, p.factor); err != nil {
			klog.Errorf("kosmos filter pv failed, pv: %v", upstreamPV.Name)
			return
		}
		if pvInstanceDeepCopy.Spec.ClaimRef != nil || upstreamPV.Spec.ClaimRef == nil {
			claim := pvInstanceDeepCopy.Spec.ClaimRef
			var newPVC *corev1.PersistentVolumeClaim
			newPVC, err = p.client.CoreV1().PersistentVolumeClaims(claim.Namespace).Get(context.TODO(), claim.Name, metav1.GetOptions{})
			if err != nil {
				klog.Errorf("kosmos sync downstream: get pvc from upstream failed, pv: %v", pvInstance.Name)
				return
			}
			pvInstanceDeepCopy.Spec.ClaimRef.UID = newPVC.UID
			pvInstanceDeepCopy.Spec.ClaimRef.ResourceVersion = newPVC.ResourceVersion
		}
		upstreamPV, err = p.client.CoreV1().PersistentVolumes().Create(context.TODO(), upstreamPV, metav1.CreateOptions{})
		if err != nil || upstreamPV == nil {
			klog.Errorf("kosmos sync downstream: create pv in upstream failed, error: %v", err)
			return
		}
		klog.Infof("kosmos sync downstream: create pv in upstream succeed, pv: %v", pv2key.(string))
	}

	if err = p.filterPV(upstreamPV, p.factor); err != nil {
		klog.Errorf("kosmos filter pv failed, pv: %v", upstreamPV.Name)
		return
	}
	if pvInstanceDeepCopy.Spec.ClaimRef != nil || upstreamPV.Spec.ClaimRef == nil {
		claim := pvInstanceDeepCopy.Spec.ClaimRef
		var newPVC *corev1.PersistentVolumeClaim
		newPVC, err = p.client.CoreV1().PersistentVolumeClaims(claim.Namespace).Get(context.TODO(), claim.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("kosmos sync downstream: get pvc from upstream failed, pv: %v", pvInstance.Name)
			return
		}
		pvInstanceDeepCopy.Spec.ClaimRef.UID = newPVC.UID
		pvInstanceDeepCopy.Spec.ClaimRef.ResourceVersion = newPVC.ResourceVersion
	}

	_, err = p.patchPV(context.TODO(), p.client, upstreamPV, pvInstanceDeepCopy)
	if err != nil {
		klog.Errorf("kosmos sync downstream: patch pv from upstream failed, pv: %v", pvInstance.Name)
		return
	}
	p.downstreamQueue.Forget(key2pv)
	klog.Infof("kosmos sync downstream: finish sync downstream pv, pv: %v", pv2key.(string))
}

func (p *PVController) deletePV(obj interface{}) {
	delPV := obj.(*corev1.PersistentVolume)
	if delPV.Namespace == metav1.NamespaceSystem || delPV.Name == "kubernetes" {
		key, err := cache.MetaNamespaceKeyFunc(p)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		p.upstreamQueue.Add(key)
		klog.Infof("kosmos delete pv, enqueue: %v", key)
	} else {
		klog.Infof("kosmos ignore pv change, pv: ", delPV.Name)
	}
}

func (p *PVController) patchPV(ctx context.Context, c kubernetes.Interface, old, new *corev1.PersistentVolume) (*corev1.PersistentVolume, error) {
	if reflect.DeepEqual(old.Annotations, new.Annotations) && reflect.DeepEqual(old.Spec, new.Spec) && reflect.DeepEqual(old.Status, new.Status) {
		return old, nil
	}

	new.Spec.NodeAffinity = old.Spec.NodeAffinity
	new.UID = old.UID
	new.ResourceVersion = old.ResourceVersion
	oldByte, err := json.Marshal(old)
	if err != nil {
		return nil, err
	}
	newByte, err := json.Marshal(new)
	if err != nil {
		return nil, err
	}
	patchByte, err := jsonpatch.CreateMergePatch(oldByte, newByte)
	if err != nil {
		return nil, err
	}

	newPV, err := c.CoreV1().PersistentVolumes().Patch(ctx, old.Name, types.MergePatchType, patchByte, metav1.PatchOptions{})
	if err != nil {
		return old, err
	}

	return newPV, nil
}

func (p *PVController) filterPV(pv *corev1.PersistentVolume, factor string) error {
	utils.TrimObjectMeta(&pv.ObjectMeta)
	if pv.Spec.NodeAffinity == nil {
		return fmt.Errorf("kosmos filter pv failed, pv: %v", pv.Name)
	}
	if pv.Spec.NodeAffinity.Required == nil {
		return fmt.Errorf("kosmos filter pv failed, pv: %v", pv.Name)
	}

	nsTerms := pv.Spec.NodeAffinity.Required.NodeSelectorTerms
	for k, v := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		mFields := v.MatchFields
		mExpressions := v.MatchExpressions
		for k1, v1 := range v.MatchFields {
			if v1.Key == utils.NodeHostnameValue || v1.Key == utils.NodeHostnameValueBeta {
				v1.Values = []string{factor}
			}
			mFields[k1] = v1
		}
		for k2, v2 := range v.MatchExpressions {
			if v2.Key == utils.NodeHostnameValue || v2.Key == utils.NodeHostnameValueBeta {
				v2.Values = []string{factor}
			}
			mExpressions[k2] = v2
		}
		nsTerms[k].MatchFields = mFields
		nsTerms[k].MatchExpressions = mExpressions
	}
	pv.Spec.NodeAffinity.Required.NodeSelectorTerms = nsTerms
	return nil
}
