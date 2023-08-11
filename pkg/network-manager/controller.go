package network_manager

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/network-manager/handlers"
)

const (
	ControllerName = "node-config-controller"
	RequeueTime    = 10 * time.Second
)

type Controller struct {
	client.Client
	EventRecorder record.EventRecorder

	NetworkManager *Manager

	sync.RWMutex
}

var predicatesFunc = predicate.Funcs{
	CreateFunc: func(createEvent event.CreateEvent) bool {
		return true
	},
	UpdateFunc: func(updateEvent event.UpdateEvent) bool {
		return true
	},
	DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
		return true
	},
	GenericFunc: func(genericEvent event.GenericEvent) bool {
		return false
	},
}

func (c *Controller) newClusterMapFunc() handler.MapFunc {
	return func(a client.Object) []reconcile.Request {
		var requests []reconcile.Request
		cluster := a.(*clusterlinkv1alpha1.Cluster)

		clusterNodeList := &clusterlinkv1alpha1.ClusterNodeList{}
		if err := c.Client.List(context.TODO(), clusterNodeList); err != nil {
			klog.Errorf("failed to list cluster nodes, error: %v", err)
			return nil
		}

		for _, node := range clusterNodeList.Items {
			if node.Spec.ClusterName == cluster.Name && node.IsGateway() {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
					Name: node.Name,
				}})
				break
			}
		}

		return requests
	}
}

func (c *Controller) SetupWithManager(mgr manager.Manager) error {

	if c.NetworkManager == nil {
		c.NetworkManager = NewManager()
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{}).
		For(&clusterlinkv1alpha1.ClusterNode{}, builder.WithPredicates(predicatesFunc)).
		Watches(&source.Kind{Type: &clusterlinkv1alpha1.Cluster{}}, handler.EnqueueRequestsFromMapFunc(c.newClusterMapFunc())).
		Complete(c)
}

func (c *Controller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {

	c.Lock()
	defer c.Unlock()

	klog.V(4).Infof("============ %s starts to reconcile %s ============", ControllerName, request.NamespacedName)

	clusterNodeList := &clusterlinkv1alpha1.ClusterNodeList{}
	if err := c.Client.List(ctx, clusterNodeList); err != nil {
		klog.Errorf("failed to list cluster nodes, error: %v", err)
		return reconcile.Result{RequeueAfter: RequeueTime}, nil
	}

	clusterList := &clusterlinkv1alpha1.ClusterList{}
	if err := c.Client.List(ctx, clusterList); err != nil {
		klog.Errorf("failed to list clusters, error: %v", err)
		return reconcile.Result{RequeueAfter: RequeueTime}, nil
	}

	nodeConfigList := &clusterlinkv1alpha1.NodeConfigList{}
	if err := c.Client.List(ctx, nodeConfigList); err != nil {
		klog.Errorf("failed to list nodeConfigs, error: %v", err)
		return reconcile.Result{RequeueAfter: RequeueTime}, nil
	}

	nodeConfigs, err := c.NetworkManager.CalculateNetworkConfigs(clusterList.Items, clusterNodeList.Items, nodeConfigList.Items)
	if err != nil {
		klog.Errorf("failed to calculate network configs, error: %v", err)
		return reconcile.Result{RequeueAfter: RequeueTime}, nil
	}

	str := c.NetworkManager.GetConfigsString()
	klog.V(4).Infof(str)

	for nodeName, config := range nodeConfigs {
		nc := &clusterlinkv1alpha1.NodeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
		}
		results, err := controllerutil.CreateOrUpdate(ctx, c.Client, nc, c.mutateNodeConfig(nc, config))
		if err != nil {
			klog.Infof("failed to create or update nodeConfig, will requeue. err: %v, ", err)
			return reconcile.Result{RequeueAfter: RequeueTime}, err
		}
		klog.V(4).Infof("successfully created or updated nodeConfig %s, results: %s", nodeName, results)
	}

	return reconcile.Result{}, nil
}

func (c *Controller) mutateNodeConfig(nc *clusterlinkv1alpha1.NodeConfig, config *handlers.NodeConfig) controllerutil.MutateFn {
	return func() error {
		strOld, err := json.Marshal(nc.Spec)
		if err != nil {
			klog.Errorf("Marshal NodeConfig.spec err: %v", err)
			return err
		}
		specNew := config.ConvertToNodeConfigSpec()
		nc.SetAnnotations(map[string]string{
			"prev": string(strOld),
		})
		nc.Spec = specNew
		return nil
	}
}
