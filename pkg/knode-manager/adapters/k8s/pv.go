package k8sadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	jsonpatch "github.com/evanphx/json-patch"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/knode-manager/utils"
)

type PVAdapter struct {
	master kubernetes.Interface
	client kubernetes.Interface

	updatedPV chan *corev1.PersistentVolume

	stopCh <-chan struct{}
}

func NewPVAdapter(ctx context.Context, ac *AdapterConfig) (*PVAdapter, error) {
	adapter := &PVAdapter{
		master: ac.Master,
		client: ac.Client,
		stopCh: ctx.Done(),
	}

	cInformer := kubeinformers.NewSharedInformerFactory(adapter.client, 0).Core().V1().PersistentVolumes()
	_, _ = cInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    adapter.addPV,
		UpdateFunc: adapter.updatePV,
	})

	return adapter, nil
}

func (p *PVAdapter) addPV(obj interface{}) {
	pv := obj.(*corev1.PersistentVolume)
	if err := p.syncAnnotation(pv); err != nil {
		return
	}

	if pv.Annotations == nil || pv.Annotations["kosmos.io/global"] != "true" {
		return
	}

	if pv.Namespace == metav1.NamespaceSystem || pv.Name == "kubernetes" {
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		klog.Info("kosmos add pv, enqueue: %v", key)
		p.updatedPV <- pv
	} else {
		klog.Infof("kosmos ignore pv change, pv: ", pv.Name)
	}
}

func (p *PVAdapter) updatePV(oldObj, newObj interface{}) {
	newPV := newObj.(*corev1.PersistentVolume)
	if err := p.syncAnnotation(newPV); err != nil {
		return
	}

	if newPV.Annotations == nil || newPV.Annotations["kosmos.io/global"] != "true" {
		return
	}

	if newPV.Namespace == metav1.NamespaceSystem || newPV.Name == "kubernetes" {
		key, err := cache.MetaNamespaceKeyFunc(newObj)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		klog.Info("kosmos update pv, enqueue: %v", key)
		p.updatedPV <- newPV
	} else {
		klog.Infof("kosmos ignore pv change, pv: ", newPV.Name)
	}
}

func (p *PVAdapter) syncAnnotation(newPV *corev1.PersistentVolume) error {
	if newPV.Status.Phase == corev1.VolumeBound {
		pvCopy := newPV.DeepCopy()
		if pvCopy.Annotations == nil {
			pvCopy.Annotations = make(map[string]string)
		}
		pvcRef := pvCopy.Spec.ClaimRef
		pvc, err := p.client.CoreV1().PersistentVolumeClaims(pvcRef.Namespace).Get(context.TODO(), pvcRef.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("kosmos sync annotation: get pvc from downstream failed, error: %v", err)
			return err
		}
		if pvc.Annotations == nil || pvc.Annotations["kosmos.io/global"] != "true" {
			if pvc.Annotations == nil {
				pvc.Annotations = map[string]string{}
			}
			pvc.Annotations["kosmos.io/global"] = "true"
			_, err = p.Patch(context.TODO(), newPV, pvCopy)
			if err != nil {
				klog.Errorf("kosmos sync annotation: patch pv from downstream failed, error: %v", err)
				return err
			}
			return nil
		}
		klog.Infof("kosmos sync annotation: skip sync annotation")
	}
	return nil
}

func (p *PVAdapter) Get(ctx context.Context, name string) (*corev1.PersistentVolume, error) {
	pv, err := p.client.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		if apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("kosmos could not get pv %s: %v", name, err)
	}
	pvCopy := pv.DeepCopy()
	utils.RecoverLabels(pvCopy.Labels, pvCopy.Annotations)
	return pvCopy, nil
}

func (p *PVAdapter) Patch(ctx context.Context, old, new *corev1.PersistentVolume) (*corev1.PersistentVolume, error) {
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

	newPV, err := p.client.CoreV1().PersistentVolumes().Patch(ctx, old.Name, types.MergePatchType, patchByte, metav1.PatchOptions{})
	if err != nil {
		return old, err
	}

	return newPV, nil
}

func (p *PVAdapter) Delete(ctx context.Context, pv *corev1.PersistentVolume) error {
	err := p.client.CoreV1().PersistentVolumes().Delete(ctx, pv.Name, metav1.DeleteOptions{})
	if !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (p *PVAdapter) Notify(ctx context.Context, f func(*corev1.PersistentVolume)) {
	klog.Info("Called NotifyPVs")
	go func() {
		for {
			select {
			case pv := <-p.updatedPV:
				klog.Infof("Enqueue updated pv %v", pv.Name)
				f(pv)
			case <-p.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}
