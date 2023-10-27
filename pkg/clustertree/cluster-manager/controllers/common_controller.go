package controllers

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

const SyncResourcesRequeueTime = 10 * time.Second

var SYNC_GVRS = []schema.GroupVersionResource{utils.GVR_CONFIGMAP, utils.GVR_SECRET}
var SYNC_OBJS = []client.Object{&corev1.ConfigMap{}, &corev1.Secret{}}

const SYNC_KIND_CONFIGMAP = "ConfigMap"
const SYNC_KIND_SECRET = "SECRET"

type SyncResourcesReconciler struct {
	GroupVersionResource schema.GroupVersionResource
	Object               client.Object
	DynamicLeafClient    dynamic.Interface
	DynamicRootClient    dynamic.Interface
	ControllerName       string

	client.Client
	LeafClient client.Client
	RootClient client.Client
	Namespace  string
}

func (r *SyncResourcesReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// skip namespace
	if len(r.Namespace) > 0 && r.Namespace != request.Namespace {
		return reconcile.Result{}, nil
	}

	_, err := r.DynamicRootClient.Resource(r.GroupVersionResource).Namespace(request.Namespace).Get(ctx, request.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: SyncResourcesRequeueTime}, nil
	}

	if err = r.SyncResource(ctx, request); err != nil {
		klog.Errorf("sync resource %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: SyncResourcesRequeueTime}, nil
	}

	return reconcile.Result{}, nil
}

func (r *SyncResourcesReconciler) SetupWithManager(mgr manager.Manager, gvr schema.GroupVersionResource) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		Named(r.ControllerName).
		WithOptions(controller.Options{}).
		For(r.Object, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				return true
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return true
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return true
			},
		})).
		Complete(r); err != nil {
		return err
	}
	return nil
}

func (r *SyncResourcesReconciler) SyncResource(ctx context.Context, request reconcile.Request) error {
	klog.V(4).Infof("Started sync resource processing, ns: %s, name: %s", request.Namespace, request.Name)

	deleteSecretInClient := false

	obj, err := r.DynamicRootClient.Resource(r.GroupVersionResource).Namespace(request.Namespace).Get(ctx, request.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// get obj in leaf cluster
		_, err := r.DynamicLeafClient.Resource(r.GroupVersionResource).Namespace(request.Namespace).Get(ctx, request.Name, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("Get %s from leaef cluster failed, error: %v", obj.GetKind(), err)
				return err
			}
			return nil
		}

		// delete OBJ in leaf cluster
		deleteSecretInClient = true
	}

	if deleteSecretInClient || obj.GetDeletionTimestamp() != nil {
		// delete OBJ in leaf cluster
		if err = r.DynamicLeafClient.Resource(r.GroupVersionResource).Namespace(request.Namespace).Delete(ctx, obj.GetName(), metav1.DeleteOptions{}); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		klog.V(3).Infof("%s %q deleted", obj.GetKind(), obj.GetName())
		return nil
	}

	old, err := r.DynamicLeafClient.Resource(r.GroupVersionResource).Namespace(request.Namespace).Get(ctx, request.Name, metav1.GetOptions{})

	if err != nil {
		if errors.IsNotFound(err) {
			// TODO: maybe deleted in leaf cluster by other people
			klog.Errorf("Get %s from client cluster failed when try to update , error: %v", obj.GetKind(), err)
			return nil
		}
		klog.Errorf("Get %s from client cluster failed, error: %v", obj.GetKind(), err)
		return err
	}

	var latest *unstructured.Unstructured
	var unstructerr error
	switch old.GetKind() {
	case SYNC_KIND_CONFIGMAP:
		latest, unstructerr = utils.UpdateUnstructured(old, obj, &corev1.ConfigMap{}, &corev1.ConfigMap{}, utils.UpdateConfigMap)
	case SYNC_KIND_SECRET:
		latest, unstructerr = utils.UpdateUnstructured(old, obj, &corev1.Secret{}, &corev1.Secret{}, utils.UpdateSecret)
	}

	if unstructerr != nil {
		return unstructerr
	}
	if !utils.IsObjectUnstructuredGlobal(old.GetAnnotations()) {
		return nil
	}
	_, err = r.DynamicLeafClient.Resource(r.GroupVersionResource).Namespace(request.Namespace).Update(ctx, latest, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("update %s from client cluster failed, error: %v", latest.GetKind(), err)
		return err
	}
	return nil
}
