package k8sadapter

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/knode-manager/utils"
)

type PVCAdapter struct {
	master kubernetes.Interface
	client kubernetes.Interface

	updatedPVC chan *corev1.PersistentVolumeClaim

	stopCh <-chan struct{}
}

func NewPVCAdapter(ctx context.Context, ac *AdapterConfig) (*PVCAdapter, error) {
	adapter := &PVCAdapter{
		master: ac.Master,
		client: ac.Client,
		stopCh: ctx.Done(),
	}

	cInformer := kubeinformers.NewSharedInformerFactory(adapter.client, 0).Core().V1().PersistentVolumeClaims()
	_, _ = cInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: adapter.updatePVC,
	})

	return adapter, nil
}

func (p *PVCAdapter) updatePVC(oldObj, newObj interface{}) {
	newPVC := newObj.(*corev1.PersistentVolumeClaim)
	if newPVC.Annotations == nil || newPVC.Annotations["kosmos.io/global"] != "true" {
		return
	}

	if newPVC.Namespace == metav1.NamespaceSystem || newPVC.Name == "kubernetes" {
		key, err := cache.MetaNamespaceKeyFunc(newObj)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		klog.Info("kosmos update pvc, enqueue: %v", key)
		p.updatedPVC <- newPVC
	} else {
		klog.Infof("kosmos ignore pvc change, pvc: ", newPVC.Name)
	}
}

func (p *PVCAdapter) Get(ctx context.Context, ns, name string) (*corev1.PersistentVolumeClaim, error) {
	pvc, err := p.client.CoreV1().PersistentVolumeClaims(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		if apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("kosmos could not get pvc %s: %v", name, err)
	}
	pvcCopy := pvc.DeepCopy()
	utils.RecoverLabels(pvcCopy.Labels, pvcCopy.Annotations)
	return pvcCopy, nil
}

func (p *PVCAdapter) Delete(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	err := p.client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(ctx, pvc.Name, metav1.DeleteOptions{})
	if !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (p *PVCAdapter) Notify(ctx context.Context, f func(*corev1.PersistentVolumeClaim)) {
	klog.Info("Called NotifyPVCs")
	go func() {
		for {
			select {
			case pvc := <-p.updatedPVC:
				klog.Infof("Enqueue updated pvc %v", pvc.Name)
				f(pvc)
			case <-p.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}
