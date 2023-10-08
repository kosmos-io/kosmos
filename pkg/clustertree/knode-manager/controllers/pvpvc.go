package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	mergetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/utils"
)

const ComponentName = "pv-pvc-controller"

type PVPVCController struct {
	master        kubernetes.Interface
	client        kubernetes.Interface
	eventRecorder record.EventRecorder

	clientPVCQueue          workqueue.RateLimitingInterface
	clientPVQueue           workqueue.RateLimitingInterface
	clientPVCLister         corelisters.PersistentVolumeClaimLister
	clientPVCInformerSynced cache.InformerSynced
	clientPVLister          corelisters.PersistentVolumeLister
	clientPVInformerSynced  cache.InformerSynced

	masterPVCQueue          workqueue.RateLimitingInterface
	masterPVQueue           workqueue.RateLimitingInterface
	masterPVCLister         corelisters.PersistentVolumeClaimLister
	masterPVCInformerSynced cache.InformerSynced
	masterPVLister          corelisters.PersistentVolumeLister
	masterPVInformerSynced  cache.InformerSynced

	nodeName string
}

func NewPVPVCController(master kubernetes.Interface, client kubernetes.Interface,
	masterInformer, clientInformer informers.SharedInformerFactory, nodeName string) (*PVPVCController, error) {
	c := &PVPVCController{
		master:         master,
		client:         client,
		clientPVCQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		clientPVQueue:  workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		masterPVCQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		masterPVQueue:  workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		nodeName:       nodeName,
	}

	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: master.CoreV1().Events(v1.NamespaceAll)})
	eventRecorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: ComponentName})
	c.eventRecorder = eventRecorder

	// master
	pvcInformer := masterInformer.Core().V1().PersistentVolumeClaims()
	pvInformer := masterInformer.Core().V1().PersistentVolumes()

	c.masterPVCLister = pvcInformer.Lister()
	c.masterPVCInformerSynced = pvcInformer.Informer().HasSynced
	c.masterPVLister = pvInformer.Lister()
	c.masterPVInformerSynced = pvInformer.Informer().HasSynced

	_, err := pvcInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.masterPVCUpdated,
		DeleteFunc: c.masterPVCDeleted,
	})
	if err != nil {
		return nil, err
	}
	_, err = pvInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.masterPVDeleted,
	})
	if err != nil {
		return nil, err
	}

	// client
	clientPVCInformer := clientInformer.Core().V1().PersistentVolumeClaims()
	clientPVInformer := clientInformer.Core().V1().PersistentVolumes()

	c.clientPVCLister = clientPVCInformer.Lister()
	c.clientPVCInformerSynced = clientPVCInformer.Informer().HasSynced
	c.clientPVLister = clientPVInformer.Lister()
	c.clientPVInformerSynced = clientPVInformer.Informer().HasSynced

	_, err = clientPVCInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.clientPVCUpdated,
	})
	if err != nil {
		return nil, err
	}
	_, err = clientPVInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.clientPVAdded,
		UpdateFunc: c.clientPVUpdated,
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *PVPVCController) Run(ctx context.Context) error {
	defer c.clientPVCQueue.ShutDown()
	defer c.clientPVQueue.ShutDown()
	defer c.masterPVCQueue.ShutDown()
	defer c.masterPVQueue.ShutDown()

	klog.Infof("Starting %s", ComponentName)
	defer klog.Infof("Shutting %s", ComponentName)

	stopCh := ctx.Done()
	workers := 1

	if !cache.WaitForCacheSync(stopCh, c.masterPVInformerSynced, c.masterPVCInformerSynced) {
		return fmt.Errorf("cannot sync pv pvc caches from master")
	}
	klog.Infof("Master pv pvc caches synced")

	if !cache.WaitForCacheSync(stopCh, c.clientPVInformerSynced, c.clientPVCInformerSynced) {
		return fmt.Errorf("cannot sync pv pvc caches from client")
	}
	klog.Infof("Client pv pvc caches synced")

	//go c.runGC(ctx)
	c.gc(ctx)

	for i := 0; i < workers; i++ {
		go wait.Until(c.syncMasterPVC, 0, stopCh)
		go wait.Until(c.syncMasterPV, 0, stopCh)
		go wait.Until(c.syncClientPVC, 0, stopCh)
		go wait.Until(c.syncClientPV, 0, stopCh)
	}

	<-stopCh
	return nil
}

func (c *PVPVCController) syncMasterPVC() {
	keyObj, quit := c.masterPVCQueue.Get()
	if quit {
		return
	}
	defer c.masterPVCQueue.Done(keyObj)

	var err error
	defer func() {
		if err != nil {
			c.masterPVCQueue.AddRateLimited(key)
			return
		}
		c.masterPVCQueue.Forget(key)
	}()

	key := keyObj.(string)
	namespace, pvcName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.masterPVCQueue.Forget(key)
		err = nil
		return
	}
	klog.V(4).Infof("Started master pvc processing %q", pvcName)

	var pvc *v1.PersistentVolumeClaim
	deletePVCInClient := false
	pvc, err = c.masterPVCLister.PersistentVolumeClaims(namespace).Get(pvcName)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return
		}
		_, err = c.clientPVCLister.PersistentVolumeClaims(namespace).Get(pvcName)
		if err != nil {
			if !apierrs.IsNotFound(err) {
				klog.Errorf("Get pvc from master cluster failed, error: %v", err)
				return
			}
			err = nil
			klog.V(4).Infof("Client pvc %q has deleted", pvcName)
			return
		}
		deletePVCInClient = true
	}

	if deletePVCInClient || pvc.DeletionTimestamp != nil {
		if err = c.client.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), pvcName,
			metav1.DeleteOptions{}); err != nil {
			if !apierrs.IsNotFound(err) {
				klog.Errorf("Delete pvc from client cluster failed, error: %v", err)
				return
			}
			err = nil
		}
		klog.V(4).Infof("Client pvc %q has deleted", pvcName)
		return
	}

	// TODO Check whether the nodeName of the pod associated with pvc is the current knode
	var old *v1.PersistentVolumeClaim
	old, err = c.clientPVCLister.PersistentVolumeClaims(namespace).Get(pvcName)
	if err != nil {
		klog.Warningf("Get pvc from client cluster failed, error: %v", err)
		return
	}

	_, err = c.patchPVC(old, pvc, c.client, false)
	if err != nil {
		klog.Errorf("Get pvc from client cluster failed, error: %v", err)
		return
	}
}

func (c *PVPVCController) syncMasterPV() {
	key, quit := c.masterPVQueue.Get()
	if quit {
		return
	}
	defer c.masterPVQueue.Done(key)

	pvName := key.(string)
	klog.V(4).Infof("Started master pv processing %q", pvName)
	pv, err := c.masterPVLister.Get(pvName)
	defer func() {
		if err != nil {
			c.masterPVQueue.AddRateLimited(key)
			return
		}
		c.masterPVQueue.Forget(key)
	}()
	pvNeedDelete := false
	if err != nil {
		if !apierrs.IsNotFound(err) {
			klog.Errorf("Error getting pv %q: %v", pvName, err)
			return
		}
		_, err = c.master.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
		if err != nil {
			if !apierrs.IsNotFound(err) {
				klog.Errorf("Error getting pv %q: %v", pvName, err)
				return
			}
			pvNeedDelete = true
		}
	}

	if pvNeedDelete || pv.DeletionTimestamp != nil {
		if err = c.client.CoreV1().PersistentVolumes().Delete(context.TODO(), pvName,
			metav1.DeleteOptions{}); err != nil {
			if !apierrs.IsNotFound(err) {
				klog.Errorf("Delete pvc from client cluster failed, error: %v", err)
				return
			}
			err = nil
		}
		klog.V(4).Infof("PV %q deleted", pvName)
		return
	}
}

func (c *PVPVCController) masterPVCUpdated(old, new interface{}) {
	newPVC := new.(*v1.PersistentVolumeClaim)
	if c.shouldEnqueue(&newPVC.ObjectMeta) {
		key, err := cache.MetaNamespaceKeyFunc(new)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		c.masterPVCQueue.Add(key)
		klog.V(4).Infof("Master pvc updated, key: %s", key)
	} else {
		klog.V(4).Infof("The change of master pvc %q is ignored", newPVC.Name)
	}
}

func (c *PVPVCController) masterPVCDeleted(obj interface{}) {
	pvc := obj.(*v1.PersistentVolumeClaim)
	if c.shouldEnqueue(&pvc.ObjectMeta) {
		key, err := cache.MetaNamespaceKeyFunc(pvc)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		c.masterPVCQueue.Add(key)
		klog.V(4).Infof("Master pvc deleted, key: %s", key)
	} else {
		klog.V(4).Infof("The deletion of master pvc %q is ignored", pvc.Name)
	}
}

func (c *PVPVCController) masterPVDeleted(obj interface{}) {
	pv := obj.(*v1.PersistentVolume)
	if c.shouldEnqueue(&pv.ObjectMeta) {
		key, err := cache.MetaNamespaceKeyFunc(pv)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		c.masterPVQueue.Add(key)
		klog.V(4).Infof("Master pv deleted, key: %s", key)
	} else {
		klog.V(4).Infof("The deletion of master pv %q is ignored", pv.Name)
	}
}

func (c *PVPVCController) syncClientPVC() {
	keyObj, quit := c.clientPVCQueue.Get()
	if quit {
		return
	}
	defer c.clientPVCQueue.Done(keyObj)
	key := keyObj.(string)
	namespace, pvcName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.clientPVCQueue.Forget(key)
		return
	}
	klog.V(4).Infof("Started client pvc processing %q", pvcName)

	defer func() {
		if err != nil {
			c.clientPVCQueue.AddRateLimited(key)
			return
		}
		c.clientPVCQueue.Forget(key)
	}()
	var pvc *v1.PersistentVolumeClaim
	pvc, err = c.clientPVCLister.PersistentVolumeClaims(namespace).Get(pvcName)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			klog.Errorf("Get pvc from client cluster failed, error: %v", err)
			return
		}
		err = nil
		klog.V(4).Infof("Client pvc %q deleted", pvcName)
		return
	}

	c.syncPVCToMaster(pvc)
}

func (c *PVPVCController) syncClientPV() {
	key, quit := c.clientPVQueue.Get()
	if quit {
		return
	}
	defer c.clientPVQueue.Done(key)
	pvName := key.(string)
	klog.V(4).Infof("Started client pv processing %q", pvName)

	pv, err := c.clientPVLister.Get(pvName)
	defer func() {
		if err != nil {
			c.clientPVQueue.AddRateLimited(key)
			return
		}
		c.clientPVQueue.Forget(key)
	}()
	pvNeedDelete := false
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return
		}
		err = nil
		pvNeedDelete = true
	}

	if pvNeedDelete || pv.DeletionTimestamp != nil {
		if err = c.master.CoreV1().PersistentVolumes().Delete(context.TODO(), pvName,
			metav1.DeleteOptions{}); err != nil {
			if !apierrs.IsNotFound(err) {
				klog.Errorf("Delete master pvc failed, error: %v", err)
				return
			}
			err = nil
		}
		klog.V(4).Infof("Master pv %q deleted", pvName)
		return
	}

	c.syncPVToMaster(pv)
}

func (c *PVPVCController) clientPVAdded(obj interface{}) {
	pv := obj.(*v1.PersistentVolume)
	if err := c.trySetAnnotation(pv); err != nil {
		return
	}

	if !utils.IsObjectGlobal(&pv.ObjectMeta) {
		return
	}

	if c.shouldEnqueue(&pv.ObjectMeta) {
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		klog.Info("Enqueue pv add ", "key ", key)
		c.clientPVQueue.Add(key)
	} else {
		klog.V(4).Infof("Ignoring pv %q change", pv.Name)
	}
}

func (c *PVPVCController) clientPVUpdated(old, new interface{}) {
	newPV := new.(*v1.PersistentVolume)

	if err := c.trySetAnnotation(newPV); err != nil {
		return
	}

	if !utils.IsObjectGlobal(&newPV.ObjectMeta) {
		return
	}

	if c.shouldEnqueue(&newPV.ObjectMeta) {
		key, err := cache.MetaNamespaceKeyFunc(new)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		c.clientPVQueue.Add(key)
		klog.V(4).Infof("PV update, enqueue pv: %v", key)
	} else {
		klog.V(4).Infof("Ignoring pv %q change", newPV.Name)
	}
}

func (c *PVPVCController) clientPVCUpdated(old, new interface{}) {
	newPVC := new.(*v1.PersistentVolumeClaim)
	if !utils.IsObjectGlobal(&newPVC.ObjectMeta) {
		return
	}
	if c.shouldEnqueue(&newPVC.ObjectMeta) {
		key, err := cache.MetaNamespaceKeyFunc(new)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		c.clientPVCQueue.Add(key)
		klog.V(4).Infof("client pvc updated, key: %s", key)
	} else {
		klog.V(4).Infof("ignore client pvc %q change", newPVC.Name)
	}
}

func (c *PVPVCController) syncPVCToMaster(pvc *v1.PersistentVolumeClaim) {
	key, err := cache.MetaNamespaceKeyFunc(pvc)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	defer func() {
		if err != nil {
			klog.Error(err)
			c.clientPVCQueue.AddRateLimited(key)
			return
		}
	}()
	var pvcInMaster *v1.PersistentVolumeClaim
	pvcInMaster, err = c.masterPVCLister.PersistentVolumeClaims(pvc.Namespace).Get(pvc.Name)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return
		}
		err = nil
		klog.Warningf("pvc %v has been deleted from master client", pvc.Name)
		return
	}

	pvcCopy := pvc.DeepCopy()
	if err = filterPVC(pvcCopy, c.nodeName); err != nil {
		return
	}
	pvcCopy.ResourceVersion = pvcInMaster.ResourceVersion
	klog.V(4).Infof("Old pvc %+v\n, new %+v", pvcInMaster, pvcCopy)
	if _, err = c.patchPVC(pvcInMaster, pvcCopy, c.master, true); err != nil {
		return
	}
	c.eventRecorder.Event(pvcInMaster, v1.EventTypeNormal, "Synced", "status of pvc synced successfully")
	c.clientPVCQueue.Forget(key)
	klog.V(4).Infof("Handler pvc: finished processing %q", pvc.Name)
}

func (c *PVPVCController) syncPVToMaster(pv *v1.PersistentVolume) {
	key, err := cache.MetaNamespaceKeyFunc(pv)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	defer func() {
		if err != nil {
			klog.Error(err)
			c.clientPVQueue.AddRateLimited(key)
			return
		}
	}()
	pvCopy := pv.DeepCopy()
	var pvInMaster *v1.PersistentVolume
	pvInMaster, err = c.masterPVLister.Get(pv.Name)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return
		}
		pvInMaster = pv.DeepCopy()
		filterPV(pvInMaster, c.nodeName)
		if pvCopy.Spec.ClaimRef != nil || pvInMaster.Spec.ClaimRef == nil {
			claim := pvCopy.Spec.ClaimRef
			var newPVC *v1.PersistentVolumeClaim
			newPVC, err = c.masterPVCLister.PersistentVolumeClaims(claim.Namespace).Get(claim.Name)
			if err != nil {
				return
			}
			pvInMaster.Spec.ClaimRef.UID = newPVC.UID
			pvInMaster.Spec.ClaimRef.ResourceVersion = newPVC.ResourceVersion
		}
		pvInMaster, err = c.master.CoreV1().PersistentVolumes().Create(context.TODO(),
			pvInMaster, metav1.CreateOptions{})
		if err != nil || pvInMaster == nil {
			klog.Errorf("Create pv in master cluster failed, error: %v", err)
			return
		}
		c.eventRecorder.Event(pvInMaster, v1.EventTypeNormal, "Synced",
			"pv created by knode pv controller")
		klog.Infof("Create pv %v in master cluster success", key)
		return
	}

	filterPV(pvInMaster, c.nodeName)

	if pvCopy.Spec.ClaimRef != nil || pvInMaster.Spec.ClaimRef == nil {
		claim := pvCopy.Spec.ClaimRef
		var newPVC *v1.PersistentVolumeClaim
		newPVC, err = c.masterPVCLister.PersistentVolumeClaims(claim.Namespace).Get(claim.Name)
		if err != nil {
			return
		}
		pvCopy.Spec.ClaimRef.UID = newPVC.UID
		pvCopy.Spec.ClaimRef.ResourceVersion = newPVC.ResourceVersion
	}

	klog.V(4).Infof("Old pv %+v\n, new %+v", pvInMaster, pvCopy)
	if _, err = c.patchPV(pvInMaster, pvCopy, c.master); err != nil {
		return
	}
	c.eventRecorder.Event(pvInMaster, v1.EventTypeNormal, "Synced",
		"pv status synced by knode pv controller")
	c.clientPVQueue.Forget(key)
	klog.V(4).Infof("Handler pv: finished processing %q", pvInMaster.Name)
}

func (c *PVPVCController) trySetAnnotation(newPV *v1.PersistentVolume) error {
	if newPV.Status.Phase == v1.VolumeBound {
		pvCopy := newPV.DeepCopy()
		if pvCopy.Annotations == nil {
			pvCopy.Annotations = make(map[string]string)
		}
		pvcRef := pvCopy.Spec.ClaimRef
		pvc, err := c.clientPVCLister.PersistentVolumeClaims(pvcRef.Namespace).Get(pvcRef.Name)
		if err != nil {
			return err
		}
		if utils.IsObjectGlobal(&pvc.ObjectMeta) {
			utils.SetObjectGlobal(&pvCopy.ObjectMeta)
			newPV, err = c.patchPV(newPV, pvCopy, c.client)
			if err != nil {
				klog.Errorf("Patch pv in client cluster failed, error: %v", err)
				return err
			}
			c.eventRecorder.Event(newPV, v1.EventTypeNormal, "Updated", "global annotation set successfully")
			return nil
		}
		klog.V(4).Infof("Skip set pv annotation for not a global")
	}
	return nil
}

func (c *PVPVCController) patchPVC(pvc, clone *v1.PersistentVolumeClaim,
	client kubernetes.Interface, updateToMaster bool) (*v1.PersistentVolumeClaim, error) {
	if reflect.DeepEqual(pvc.Spec, clone.Spec) &&
		reflect.DeepEqual(pvc.Status, clone.Status) {
		return pvc, nil
	}
	if !utils.CheckGlobalLabelEqual(&pvc.ObjectMeta, &clone.ObjectMeta) {
		if !updateToMaster {
			return pvc, nil
		}
	}
	patch, err := utils.CreateMergePatch(pvc, clone)
	if err != nil {
		return pvc, err
	}
	newPVC, err := client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Patch(context.TODO(),
		pvc.Name, mergetypes.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return pvc, err
	}
	return newPVC, nil
}

func (c *PVPVCController) patchPV(pv, clone *v1.PersistentVolume,
	client kubernetes.Interface) (*v1.PersistentVolume, error) {
	if reflect.DeepEqual(pv.Annotations, clone.Annotations) &&
		reflect.DeepEqual(pv.Spec, clone.Spec) &&
		reflect.DeepEqual(pv.Status, clone.Status) {
		return pv, nil
	}

	clone.Spec.NodeAffinity = pv.Spec.NodeAffinity
	clone.UID = pv.UID
	clone.ResourceVersion = pv.ResourceVersion
	patch, err := utils.CreateMergePatch(pv, clone)
	if err != nil {
		return pv, err
	}
	newPV, err := client.CoreV1().PersistentVolumes().Patch(context.TODO(), pv.Name,
		mergetypes.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return pv, err
	}
	return newPV, nil
}

func (c *PVPVCController) shouldEnqueue(obj *metav1.ObjectMeta) bool {
	return obj.Namespace != metav1.NamespaceSystem
}

func (c *PVPVCController) gc(ctx context.Context) {
	pvcs, err := c.clientPVCLister.List(labels.Everything())
	if err != nil {
		klog.Error(err)
		return
	}
	for _, pvc := range pvcs {
		if pvc == nil {
			continue
		}
		if !utils.IsObjectGlobal(&pvc.ObjectMeta) {
			continue
		}
		_, err = c.masterPVCLister.PersistentVolumeClaims(pvc.Namespace).Get(pvc.Name)
		if err != nil && apierrs.IsNotFound(err) {
			err := c.client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(ctx,
				pvc.Name, metav1.DeleteOptions{})
			if err != nil && !apierrs.IsNotFound(err) {
				klog.Error(err)
			}
			continue
		}
	}
}

// nolint:unused
func (c *PVPVCController) runGC(ctx context.Context) {
	wait.Until(func() {
		c.gc(ctx)
	}, 3*time.Minute, ctx.Done())
}

func filterPV(pvInSub *v1.PersistentVolume, nodeName string) {
	utils.TrimObjectMeta(&pvInSub.ObjectMeta)
	if pvInSub.Annotations == nil {
		pvInSub.Annotations = make(map[string]string)
	}
	if pvInSub.Spec.NodeAffinity == nil {
		return
	}
	if pvInSub.Spec.NodeAffinity.Required == nil {
		return
	}
	terms := pvInSub.Spec.NodeAffinity.Required.NodeSelectorTerms
	for k, v := range pvInSub.Spec.NodeAffinity.Required.NodeSelectorTerms {
		mf := v.MatchFields
		me := v.MatchExpressions
		for k, val := range v.MatchFields {
			if val.Key == utils.NodeHostnameValue || val.Key == utils.NodeHostnameValueBeta {
				val.Values = []string{nodeName}
			}
			mf[k] = val
		}
		for k, val := range v.MatchExpressions {
			if val.Key == utils.NodeHostnameValue || val.Key == utils.NodeHostnameValueBeta {
				val.Values = []string{nodeName}
			}
			me[k] = val
		}
		terms[k].MatchFields = mf
		terms[k].MatchExpressions = me
	}
	pvInSub.Spec.NodeAffinity.Required.NodeSelectorTerms = terms
}

func filterPVC(pvcInSub *v1.PersistentVolumeClaim, nodeName string) error {
	labelSelector := pvcInSub.Spec.Selector.DeepCopy()
	pvcInSub.Spec.Selector = nil
	utils.TrimObjectMeta(&pvcInSub.ObjectMeta)
	utils.SetObjectGlobal(&pvcInSub.ObjectMeta)
	if labelSelector != nil {
		labelStr, err := json.Marshal(labelSelector)
		if err != nil {
			return err
		}
		pvcInSub.Annotations[utils.KosmosPvcLabelSelector] = string(labelStr)
	}
	if len(pvcInSub.Annotations[utils.PVCSelectedNodeKey]) != 0 {
		pvcInSub.Annotations[utils.PVCSelectedNodeKey] = nodeName
	}
	return nil
}
