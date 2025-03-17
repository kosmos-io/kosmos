package generic

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/apis/config"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/syncer/synccontext"
)

var (
	fieldManager = "generic-syncer"
)

func NewGenericSyncerController(ctx *synccontext.SyncContext, config *config.FromHostCluster) *GenericSyncerController {
	obj := &unstructured.Unstructured{}
	obj.SetKind(config.Kind)
	obj.SetAPIVersion(config.APIVersion)

	return &GenericSyncerController{
		rootManager: ctx.RootManager,
		RootClient:  ctx.RootClient,
		LeafClient:  ctx.LeafClient,
		obj:         obj,
		gvk:         schema.FromAPIVersionAndKind(config.APIVersion, config.Kind),
		config:      config,
	}
}

// nolint
type GenericSyncerController struct {
	rootManager ctrl.Manager
	RootClient  client.Client
	LeafClient  client.Client

	obj client.Object

	gvk schema.GroupVersionKind

	config *config.FromHostCluster

	//globalLeafManager       clustertreeutils.LeafResourceManager
	//globalLeafClientManager clustertreeutils.LeafClientResourceManager
}

func (g *GenericSyncerController) SetupWithManager(mgr ctrl.Manager) error {
	if g.RootClient == nil {
		g.RootClient = mgr.GetClient()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(g.gvk.String() + "-syncer-controller").
		WithOptions(controller.Options{}).
		For(g.obj).
		Complete(g)
}

func (g *GenericSyncerController) Reconcile(ctx context.Context, vReq reconcile.Request) (res ctrl.Result, retErr error) {
	klog.V(4).Infof("============ %s -syncer-controller start to reconcile %s ============", g.gvk.String(), vReq.NamespacedName)
	rootObj := &unstructured.Unstructured{}
	rootObj.SetKind(g.config.Kind)
	rootObj.SetAPIVersion(g.config.APIVersion)

	leafObj := &unstructured.Unstructured{}
	leafObj.SetKind(g.config.Kind)
	leafObj.SetAPIVersion(g.config.APIVersion)

	err := g.RootClient.Get(ctx, vReq.NamespacedName, rootObj)
	if err != nil {
		if !k8serror.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		rootObj = nil
	}

	err = g.LeafClient.Get(ctx, vReq.NamespacedName, leafObj)
	if err != nil {
		if !k8serror.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		leafObj = nil
	}

	if rootObj != nil && leafObj != nil {
		return g.Sync(ctx, rootObj, leafObj)
	} else if rootObj != nil && leafObj == nil {
		return g.SyncDown(ctx, rootObj)
	} else if rootObj == nil && leafObj != nil {
		return g.SyncUp(ctx, leafObj)
	}

	return ctrl.Result{}, nil
}

func (g *GenericSyncerController) SyncDown(ctx context.Context, rootObj client.Object) (ctrl.Result, error) {
	klog.Infof("check rootCluster has new resources, but the leafClusters do not exist, Create CR resources %s %s/%s in leafCluster", g.config.Kind, rootObj.GetNamespace(), rootObj.GetName())
	_, err := g.ApplyPatches(ctx, rootObj, nil)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error create CR resources: %v", err)
	}
	return ctrl.Result{}, nil
}

func (g *GenericSyncerController) SyncUp(ctx context.Context, Obj client.Object) (ctrl.Result, error) {
	klog.Infof("check rootCluster has no resources, but the leafClusters exists, delete CR resources %s %s/%s in leafCluster", g.config.Kind, Obj.GetNamespace(), Obj.GetName())
	return g.DeleteObject(ctx, Obj)
}

func (g *GenericSyncerController) Sync(ctx context.Context, rootObj client.Object, leafObj client.Object) (ctrl.Result, error) {
	klog.Infof("check rootCluster has resources, and leafClusters exist, Update CR resources %s %s/%s in leafCluster", g.config.Kind, rootObj.GetNamespace(), rootObj.GetName())
	_, err := g.ApplyPatches(ctx, rootObj, leafObj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error update CR resources: %v", err)
	}
	return ctrl.Result{}, nil
}

func (g *GenericSyncerController) ApplyPatches(ctx context.Context, fromObj, toObj client.Object) (client.Object, error) {
	fromObjCopied, err := toUnstructured(fromObj)
	if err != nil {
		return nil, err
	}

	if toObj == nil {
		fromObjCopied.SetResourceVersion("")
		err = g.LeafClient.Create(ctx, fromObjCopied)
		if err != nil {
			return nil, err
		}
		return fromObjCopied, nil
	}
	// always apply object
	//klog.Infof("Apply %s during patching", fromObjCopied.GetName())
	//outObject := fromObjCopied.DeepCopy()
	//outObject.SetManagedFields(nil)
	//err = g.LeafClient.Patch(ctx, outObject, client.Apply, client.ForceOwnership, client.FieldOwner(fieldManager))
	//if err != nil {
	//	return nil, errors.Wrap(err, "apply object")
	//}

	var result *unstructured.Unstructured
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		currentToObj := &unstructured.Unstructured{}
		currentToObj.SetGroupVersionKind(toObj.GetObjectKind().GroupVersionKind())
		if err := g.LeafClient.Get(ctx, client.ObjectKeyFromObject(toObj), currentToObj); err != nil {
			return err
		}

		fromObjCopied.SetResourceVersion(currentToObj.GetResourceVersion())
		outObject := fromObjCopied.DeepCopy()
		outObject.SetManagedFields(nil)
		outObject.SetUID(currentToObj.GetUID())

		klog.Infof("Applying %s/%s", outObject.GetNamespace(), outObject.GetName())
		patchErr := g.LeafClient.Patch(ctx, outObject, client.Apply,
			client.ForceOwnership,
			client.FieldOwner(fieldManager),
		)
		if patchErr == nil {
			result = outObject
		}
		return patchErr
	})

	return result, errors.Wrap(err, "apply object after retries")
}

func toUnstructured(obj client.Object) (*unstructured.Unstructured, error) {
	fromCopied, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj.DeepCopyObject())
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: fromCopied}, nil
}

func (g *GenericSyncerController) DeleteObject(ctx context.Context, Obj client.Object) (ctrl.Result, error) {
	accessor, err := meta.Accessor(Obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	if Obj.GetNamespace() != "" {
		klog.Infof("delete rootCluster cr %s/%s, because leaf one was deleted", accessor.GetNamespace(), accessor.GetName())
	} else {
		klog.Infof("delete rootCluster cr %s, because leaf one was deleted", accessor.GetName())
	}
	err = g.LeafClient.Delete(ctx, Obj)
	if err != nil {
		if k8serror.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		if Obj.GetNamespace() != "" {
			klog.Infof("error deleting rootCluster cr %s/%s in root cluster: %v", accessor.GetNamespace(), accessor.GetName(), err)
		} else {
			klog.Infof("error deleting rootCluster cr %s in root cluster: %v", accessor.GetName(), err)
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
