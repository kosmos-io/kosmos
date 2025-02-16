package syncer

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/apis/config"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/generic"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/syncer/synccontext"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
)

const (
	ConfigMapName      = "sync-config"
	ConfigMapNamespace = "kosmos-system"
)

type SyncController struct {
	rootClient  client.Client
	RootManager ctrl.Manager

	GlobalLeafManager       leafUtils.LeafResourceManager
	GlobalLeafClientManager leafUtils.LeafClientResourceManager
}

func (r *SyncController) newSyncContext(ctx context.Context, logName string, lcr *leafUtils.LeafClientResource) *synccontext.SyncContext {
	return &synccontext.SyncContext{
		Context:        ctx,
		RootClient:     r.rootClient,
		LeafClient:     lcr.Client,
		CurrentCrdName: logName,
		RootManager:    r.RootManager,
	}
}

func (r *SyncController) Reconcile(ctx context.Context, vReq reconcile.Request) (res ctrl.Result, retErr error) {
	klog.V(4).Infof("============ generic-crd-syncer-controller start to reconcile %s ============", vReq.NamespacedName)
	defer klog.V(4).Infof("============ generic-crd-syncer-controller finish to reconcile %s ============", vReq.NamespacedName)

	var configMap corev1.ConfigMap
	err := r.rootClient.Get(ctx, vReq.NamespacedName, &configMap)
	if err != nil {
		return ctrl.Result{}, err
		//return ctrl.Result{}, err
	}

	configData, exists := configMap.Data["sync-config"]
	if !exists {
		klog.Errorf("sync-config not found in ConfigMap %s", vReq.NamespacedName)
		return ctrl.Result{}, nil
	}

	parseConfig, err := config.ParseConfig(configData)
	if err != nil {
		klog.Errorf("parse sync-config failed: %v", err)
		return ctrl.Result{}, nil
	}

	clusters := r.GlobalLeafManager.ListClusters()
	for _, cluster := range clusters {
		lcr, err := r.GlobalLeafClientManager.GetLeafResource(cluster)
		if err != nil {
			klog.Errorf("get leaf resource failed: %v", err)
		}
		registerCtx := r.newSyncContext(ctx, "", lcr)
		for _, mapping := range parseConfig.Mappings {
			if mapping.FromHostCluster != nil {
				err := leafUtils.EnsureCRDFromRootClusterToLeafCluster(ctx, registerCtx.RootManager.GetConfig(), lcr.RestConfig, schema.FromAPIVersionAndKind(mapping.FromHostCluster.APIVersion, mapping.FromHostCluster.Kind))
				if err != nil {
					klog.Errorf("ensure CRD failed: %v", err)
				}
			}
		}
	}
	for _, cluster := range clusters {
		lcr, err := r.GlobalLeafClientManager.GetLeafResource(cluster)
		if err != nil {
			klog.Errorf("get leaf resource failed: %v", err)
		}
		for _, mapping := range parseConfig.Mappings {
			if mapping.FromHostCluster != nil {
				registerCtx := r.newSyncContext(ctx, mapping.FromHostCluster.APIVersion+mapping.FromHostCluster.Kind, lcr)
				createSyncerController := generic.NewGenericSyncerController(registerCtx, mapping.FromHostCluster)
				err := createSyncerController.SetupWithManager(r.RootManager)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

func (r *SyncController) SetupWithManager(mgr manager.Manager) error {
	if r.rootClient == nil {
		r.rootClient = mgr.GetClient()
	}

	// skipFunc 仅针对特定的 ConfigMap 做过滤
	skipFunc := func(obj client.Object) bool {
		// 判断是否为指定的 ConfigMap
		if obj.GetNamespace() == ConfigMapNamespace && obj.GetName() == ConfigMapName {
			return true // 只监听这个 ConfigMap，返回 true 继续处理
		}
		return false // 其他的 ConfigMap 不处理
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("syncer-controller").
		WithOptions(controller.Options{}).
		For(&corev1.ConfigMap{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				if skipFunc(createEvent.Object) {
					// 在这里执行你需要的操作，如解析 ConfigMap 内容
					klog.Infof("ConfigMap created: %s/%s", createEvent.Object.GetNamespace(), createEvent.Object.GetName())
					return true
				}
				return false
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				if skipFunc(updateEvent.ObjectNew) {
					// 在这里执行你需要的操作，如解析 ConfigMap 内容
					klog.Infof("ConfigMap updated: %s/%s", updateEvent.ObjectNew.GetNamespace(), updateEvent.ObjectNew.GetName())
					return true
				}
				return false
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				if skipFunc(deleteEvent.Object) {
					// 在这里执行你需要的操作，如解析 ConfigMap 内容
					klog.Infof("ConfigMap deleted: %s/%s", deleteEvent.Object.GetNamespace(), deleteEvent.Object.GetName())
					return true
				}
				return false
			},
			GenericFunc: func(_ event.GenericEvent) bool {
				// TODO
				return false
			},
		})).
		Complete(r)
}
