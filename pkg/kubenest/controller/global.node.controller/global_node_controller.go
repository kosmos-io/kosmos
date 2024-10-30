package globalnodecontroller

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	env "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/env"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	NodestatusUpdatefrequency = 10 * time.Second
)

type GlobalNodeController struct {
	client.Client
	RootClientSet kubernetes.Interface
	EventRecorder record.EventRecorder
	KosmosClient  versioned.Interface
	//syncNodeStatusMux sync.Mutex
}

// compareMaps compares two map[string]string and returns true if they are equal
func compareMaps(map1, map2 map[string]string) bool {
	// If lengths are different, the maps are not equal
	if len(map1) != len(map2) {
		return false
	}

	// Iterate over map1 and check if all keys and values are present in map2
	for key, value1 := range map1 {
		if value2, ok := map2[key]; !ok || value1 != value2 {
			return false
		}
	}

	// If no discrepancies are found, the maps are equal
	return true
}

func (r *GlobalNodeController) SetupWithManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(constants.GlobalNodeControllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(&v1.Node{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				oldObj := updateEvent.ObjectOld.(*v1.Node)
				newObj := updateEvent.ObjectNew.(*v1.Node)
				return !compareMaps(oldObj.Labels, newObj.Labels)
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return false
			},
		})).
		Watches(&source.Kind{Type: &v1alpha1.GlobalNode{}}, handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
			gn := a.(*v1alpha1.GlobalNode)
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{
					Name: gn.Name,
				}},
			}
		})).
		// Watches(&source.Kind{Type: &v1.Node{}}, handler.EnqueueRequestsFromMapFunc(r.newNodeMapFunc())).
		Watches(&source.Kind{Type: &v1alpha1.VirtualCluster{}}, handler.EnqueueRequestsFromMapFunc(r.newVirtualClusterMapFunc())).
		Complete(r)
}

func (r *GlobalNodeController) newVirtualClusterMapFunc() handler.MapFunc {
	return func(a client.Object) []reconcile.Request {
		var requests []reconcile.Request
		vcluster := a.(*v1alpha1.VirtualCluster)
		if vcluster.Status.Phase != v1alpha1.Completed {
			return requests
		}
		klog.V(4).Infof("global-node-controller: virtualclusternode change to completed: %s", vcluster.Name)
		for _, nodeInfo := range vcluster.Spec.PromoteResources.NodeInfos {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
				Name: nodeInfo.NodeName,
			}})
		}
		return requests
	}
}

// func (r *GlobalNodeController) newNodeMapFunc() handler.MapFunc {
// 	return func(a client.Object) []reconcile.Request {
// 		var requests []reconcile.Request
// 		node := a.(*v1.Node)
// 		klog.V(4).Infof("global-node-controller: node change: %s", node.Name)
// 		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
// 			Name: node.Name,
// 		}})
// 		return requests
// 	}
// }

// func (r *GlobalNodeController) Start(ctx context.Context) error {
// 	// globalnodes := make([]*v1alpha1.GlobalNodeList, 0)
// 	// c.nodeLock.Lock()
// 	// for _, nodeIndex := range c.nodes {
// 	// 	nodeCopy := nodeIndex.DeepCopy()
// 	// 	nodes = append(nodes, nodeCopy)
// 	// }
// 	// c.nodeLock.Unlock()

// 	// err := c.updateNodeStatus(ctx, nodes, c.LeafNodeSelectors)
// 	// if err != nil {
// 	// 	klog.Errorf(err.Error())
// 	// }

// 	go wait.UntilWithContext(ctx, func(ctx context.Context) {

// 		r.syncNodeStatusMux.Lock()
// 		defer r.syncNodeStatusMux.Unlock()

// 		var globalNodeList v1alpha1.GlobalNodeList
// 		if err := r.List(ctx, &globalNodeList); err != nil {
// 			klog.Errorf("error listing global nodes: %v", err)
// 			return
// 		}

// 		for _, globalNode := range globalNodeList.Items {
// 			if err := r.SyncNodeStatus(ctx, &globalNode); err != nil {
// 				klog.Errorf("error syncing node status for global node %s: %v", globalNode.Name, err)
// 			}
// 		}
// 	}, 10*time.Second)

// 	<-ctx.Done()
// 	return nil
// }

// func (r *GlobalNodeController) ChooseNode(ctx context.Context, globalNode *v1alpha1.GlobalNode) (*v1alpha1.GlobalNode, error) {

// 	var updateGlobalNode *v1alpha1.GlobalNode
// 	// 如果节点状态是 "InUse"
// 	if globalNode.Spec.State == v1alpha1.NodeInUse {
// 		var virtualCluster v1alpha1.VirtualCluster
// 		if err := r.Get(ctx, types.NamespacedName{Name: globalNode.Name}, &virtualCluster); err != nil {
// 			klog.Errorf("global-node-controller: SyncNodeStatus: can not get target virtualCluster, err: %s", globalNode.Name)
// 			return nil, err
// 		}
// 		k8sClient, err := util.GenerateKubeclient(&virtualCluster)
// 		if err != nil {
// 			klog.Errorf("virtualcluster %s crd kubernetes client failed: %v", virtualCluster.Name, err)
// 			return nil, err
// 		}

// 		virtualClusterNode, err := k8sClient.CoreV1().Nodes().Get(ctx, metav1.ListOptions{})
// 		if err != nil {
// 			klog.Errorf("virtualcluster %s get virtual-cluster nodes failed: %v", virtualCluster.Name, err)
// 			return nil, err
// 		}

// 		current, err := r.KosmosClient.KosmosV1alpha1().GlobalNodes().Get(context.TODO(), virtualClusterNode.Name, metav1.GetOptions{})
// 		if err != nil {
// 			if apierrors.IsNotFound(err) {
// 				klog.Errorf("globalnode %s not found. This should not happen normally", virtualClusterNode.Name)
// 				return nil, klog.Errorf("globalNode %s not found", virtualClusterNode.Name)
// 			}
// 			klog.Errorf("failed to get globalNode %s: %v", virtualClusterNode.Name, err)
// 			return nil, err
// 		}
// 		// if the node is in use, we don't need to update the status, because the status is updated by the node controller.
// 		updateGlobalNode = current.DeepCopy()
// 		updateGlobalNode.Status.Conditions = virtualClusterNode.Status.Conditions
// 		// klog.V(4).Infof("global-node-controller: SyncState: node is in use %s, skip", globalNode.Name)
// 		// return nil
// 	} else {
// 		var targetNode v1.Node
// 		if err := r.Get(ctx, types.NamespacedName{Name: globalNode.Name}, &targetNode); err != nil {
// 			klog.Errorf("global-node-controller: SyncNodeStatus: can not get target node, err: %s", globalNode.Name)
// 			return nil, err
// 		}
// 		updateGlobalNode = globalNode.DeepCopy()
// 		updateGlobalNode.Status.Conditions = targetNode.Status.Conditions
// 	}
// 	return updateGlobalNode, nil
// }

// 将node的状态上报给globalnode,对于 STATE为reserved和free的直接更新->global_node_controller
// occupied的需要获取vc的node信息再更新->node_controller
func (r *GlobalNodeController) SyncNodeStatus(ctx context.Context, globalNode *v1alpha1.GlobalNode) error {

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {

		//每次重试时重新获取最新的 globalNode
		currentGlobalNode := &v1alpha1.GlobalNode{}
		if err := r.Get(ctx, types.NamespacedName{Name: globalNode.Name}, currentGlobalNode); err != nil {
			klog.Errorf("global-node-controller: SyncNodeStatus: failed to get globalNode %s, err: %v", globalNode.Name, err)
			return err
		}

		var updateGlobalNode *v1alpha1.GlobalNode
		if globalNode.Spec.State == v1alpha1.NodeInUse {
			var virtualCluster v1alpha1.VirtualCluster
			if err := r.Get(ctx, types.NamespacedName{Name: globalNode.Name}, &virtualCluster); err != nil {
				klog.Errorf("global-node-controller: SyncNodeStatus: can not get target virtualCluster, err: %s", globalNode.Name)
				return err
			}
			k8sClient, err := util.GenerateKubeclient(&virtualCluster)
			if err != nil {
				klog.Errorf("virtualcluster %s crd kubernetes client failed: %v", virtualCluster.Name, err)
				return err
			}
			virtualClusterNode, err := k8sClient.CoreV1().Nodes().Get(ctx, globalNode.Name, metav1.GetOptions{})
			if err != nil {
				klog.Errorf("virtualcluster %s get virtual-cluster nodes failed: %v", virtualCluster.Name, err)
				return err
			}
			current, err := r.KosmosClient.KosmosV1alpha1().GlobalNodes().Get(context.TODO(), virtualClusterNode.Name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.Errorf("globalnode %s not found. This should not happen normally", virtualClusterNode.Name)
					return nil
				}
				klog.Errorf("failed to get globalNode %s: %v", virtualClusterNode.Name, err)
				return err
			}
			// if the node is in use, we don't need to update the status, because the status is updated by the node controller.
			updateGlobalNode = current.DeepCopy()
			updateGlobalNode.Status.Conditions = virtualClusterNode.Status.Conditions
			// klog.V(4).Infof("global-node-controller: SyncState: node is in use %s, skip", globalNode.Name)
			// return nil
		} else {
			var targetNode v1.Node
			if err := r.Get(ctx, types.NamespacedName{Name: globalNode.Name}, &targetNode); err != nil {
				klog.Errorf("global-node-controller: SyncNodeStatus: can not get target node, err: %s", globalNode.Name)
				return err
			}
			//updateGlobalNode = globalNode.DeepCopy()
			updateGlobalNode = currentGlobalNode.DeepCopy()
			updateGlobalNode.Status.Conditions = targetNode.Status.Conditions
		}

		if err := r.Status().Update(ctx, updateGlobalNode); err != nil {
			//klog.Errorf("update node status for globalnode failed, err: %s", globalNode.Name)
			klog.Errorf("update node %s status for globalnode failed, %v", globalNode.Name, err)
			return err
		}
		klog.V(4).Infof("SyncNodeStatus: successfully updated global node %s, Status.Conditions: %+v", updateGlobalNode.Name, updateGlobalNode.Status.Conditions)
		return nil
	})

	// err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
	// 	var targetNode v1.Node
	// 	if err := r.Get(ctx, types.NamespacedName{Name: globalNode.Name}, &targetNode); err != nil {
	// 		klog.Errorf("global-node-controller: SyncNodeStatus: can not get target node, err: %s", globalNode.Name)
	// 		return err
	// 	}

	// 	updateGlobalNode := globalNode.DeepCopy()
	// 	updateGlobalNode.Status.Conditions = targetNode.Status.Conditions

	// 	if err := r.Status().Update(ctx, updateGlobalNode); err != nil {
	// 		klog.Errorf("update node status for globalnode failed, err: %s", globalNode.Name)
	// 		return err
	// 	}
	// 	klog.V(4).Infof("SyncNodeStatus: successfully updated global node %s, Status.Conditions: %+v", updateGlobalNode.Name, updateGlobalNode.Status.Conditions)
	// 	// // 重新获取更新后的 GlobalNode，确保获取最新状态
	// rGlobalNode, err := r.KosmosClient.KosmosV1alpha1().GlobalNodes().Get(ctx, updateGlobalNode.Name, metav1.GetOptions{})
	// if err != nil {
	// 	klog.Errorf("global-node-controller: SyncLabel: failed to get updated global node: %s, err: %s", updateGlobalNode.Name, err)
	// 	return err
	// }
	// // 只打印更新后的 GlobalNode 的 Status 部分
	// klog.V(4).Infof("global-node-controller: SyncLabel: successfully updated global node %s, updated status: %+v", rGlobalNode.Name, rGlobalNode.Status)
	// 	return nil
	// })
	return err
}

func (r *GlobalNodeController) SyncTaint(ctx context.Context, globalNode *v1alpha1.GlobalNode) error {
	if globalNode.Spec.State == v1alpha1.NodeFreeState {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			//获得globalnode对应的node
			var targetNode v1.Node
			if err := r.Get(ctx, types.NamespacedName{Name: globalNode.Name}, &targetNode); err != nil {
				klog.Errorf("global-node-controller: SyncTaints: can not get host node, err: %s", globalNode.Name)
				return err
			}

			if targetNode.Spec.Unschedulable {
				klog.V(4).Infof("global-node-controller: SyncTaints: node is unschedulable %s, skip", globalNode.Name)
				return nil
			}

			if _, ok := targetNode.Labels[env.GetControlPlaneLabel()]; ok {
				klog.V(4).Infof("global-node-controller: SyncTaints: control-plane node %s, skip", globalNode.Name)
				return nil
			}

			return util.DrainNode(ctx, targetNode.Name, r.RootClientSet, &targetNode, env.GetDrainWaitSeconds(), true)
		})
		return err
	}
	klog.V(4).Infof("global-node-controller: SyncTaints: node status is %s, skip", globalNode.Spec.State, globalNode.Name)
	return nil
}

func (r *GlobalNodeController) SyncState(ctx context.Context, globalNode *v1alpha1.GlobalNode) error {
	if globalNode.Spec.State == v1alpha1.NodeInUse {
		klog.V(4).Infof("global-node-controller: SyncState: node is in use %s, skip", globalNode.Name)
		return nil
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var hostNode v1.Node
		if err := r.Get(ctx, types.NamespacedName{Name: globalNode.Name}, &hostNode); err != nil {
			klog.Errorf("global-node-controller: SyncState: can not get global node, err: %s", globalNode.Name)
			return err
		}

		updateHostNode := hostNode.DeepCopy()

		v, ok := updateHostNode.Labels[constants.StateLabelKey]
		if ok && v == string(globalNode.Spec.State) {
			return nil
		}

		updateHostNode.Labels[constants.StateLabelKey] = string(globalNode.Spec.State)
		if err := r.Update(ctx, updateHostNode); err != nil {
			klog.Errorf("global-node-controller: SyncState: update node label failed, err: %s", globalNode.Name)
			return err
		}
		return nil
	})
	return err
}

func (r *GlobalNodeController) SyncLabel(ctx context.Context, globalNode *v1alpha1.GlobalNode) error {
	if globalNode.Spec.State == v1alpha1.NodeInUse {
		klog.V(4).Infof("global-node-controller: SyncLabel: node is in use %s, skip", globalNode.Name)
		return nil
	}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		rootNode, err := r.RootClientSet.CoreV1().Nodes().Get(ctx, globalNode.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("global-node-controller: SyncLabel: can not get root node: %s", globalNode.Name)
			return err
		}

		if _, err = r.KosmosClient.KosmosV1alpha1().GlobalNodes().Get(ctx, globalNode.Name, metav1.GetOptions{}); err != nil {
			klog.Errorf("global-node-controller: SyncLabel: can not get global node: %s", globalNode.Name)
			return err
		}

		// Use management plane node label to override global node
		updateGlobalNode := globalNode.DeepCopy()
		if compareMaps(updateGlobalNode.Spec.Labels, rootNode.Labels) {
			return nil
		}
		updateGlobalNode.Spec.Labels = rootNode.Labels

		if _, err = r.KosmosClient.KosmosV1alpha1().GlobalNodes().Update(ctx, updateGlobalNode, metav1.UpdateOptions{}); err != nil {
			klog.Errorf("global-node-controller: SyncLabel: update global node label failed, err: %s", err)
			return err
		}
		return nil
	})
	return err
}

func (r *GlobalNodeController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ global-node-controller start to reconcile %s ============", request.NamespacedName)
	defer klog.V(4).Infof("============ global-node-controller finish to reconcile %s ============", request.NamespacedName)

	// r.syncNodeStatusMux.Lock()
	// defer r.syncNodeStatusMux.Unlock()

	var globalNode v1alpha1.GlobalNode
	if err := r.Get(ctx, request.NamespacedName, &globalNode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("global-node-controller: can not found %s", request.NamespacedName)
			// If global node does not found, create it
			var rootNode *v1.Node
			if rootNode, err = r.RootClientSet.CoreV1().Nodes().Get(ctx, request.Name, metav1.GetOptions{}); err != nil {
				klog.Errorf("global-node-controller: can not found root node: %s", request.Name)
				return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
			}
			globalNode.Name = request.Name
			globalNode.Spec.State = v1alpha1.NodeReserved
			firstNodeIP, err := utils.FindFirstNodeIPAddress(*rootNode, constants.PreferredAddressType)
			if err != nil {
				klog.Errorf("get first node ip address err: %s %s", constants.PreferredAddressType, err.Error())
			}
			globalNode.Spec.NodeIP = firstNodeIP
			if _, err = r.KosmosClient.KosmosV1alpha1().GlobalNodes().Create(ctx, &globalNode, metav1.CreateOptions{}); err != nil {
				klog.Errorf("global-node-controller: can not create global node: %s", globalNode.Name)
				return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
			}
			klog.V(4).Infof("global-node-controller: %s has been created", globalNode.Name)
			// do sync label and taint
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}
		klog.Errorf("get global-node %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}
	// if err := r.SyncState(ctx, &globalNode); err != nil {
	// 	klog.Errorf("sync State %s error: %v", request.NamespacedName, err)
	// 	return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	// } else {
	// 	klog.V(4).Infof("sync state successed, %s", request.NamespacedName)
	// }

	_, err := r.RootClientSet.CoreV1().Nodes().Get(ctx, globalNode.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		klog.Errorf("can not get root node: %s", globalNode.Name)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}
	if globalNode.Spec.State == v1alpha1.NodeInUse {
		// wait globalNode free
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	if err = r.SyncNodeStatus(ctx, &globalNode); err != nil {
		klog.Warningf("sync status %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}
	klog.V(4).Infof("sync status successed, %s", request.NamespacedName)

	if err = r.SyncLabel(ctx, &globalNode); err != nil {
		klog.Warningf("sync label %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}
	klog.V(4).Infof("sync label successed, %s", request.NamespacedName)

	if err = r.SyncTaint(ctx, &globalNode); err != nil {
		klog.Errorf("sync taint %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}
	klog.V(4).Infof("sync taint successed, %s", request.NamespacedName)

	return reconcile.Result{}, nil
}
