package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	env "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/env"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/exector"
	"github.com/kosmos.io/kosmos/pkg/kubenest/tasks"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	apiclient "github.com/kosmos.io/kosmos/pkg/kubenest/util/api-client"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type VirtualClusterInitController struct {
	client.Client
	Config          *rest.Config
	EventRecorder   record.EventRecorder
	RootClientSet   kubernetes.Interface
	KosmosClient    versioned.Interface
	lock            sync.Mutex
	KubeNestOptions *v1alpha1.KubeNestConfiguration
}

type NodePool struct {
	Address string            `json:"address" yaml:"address"`
	Labels  map[string]string `json:"labels" yaml:"labels"`
	Cluster string            `json:"cluster" yaml:"cluster"`
	State   string            `json:"state" yaml:"state"`
}

type HostPortPool struct {
	PortsPool []int32 `yaml:"portsPool"`
}

type VipPool struct {
	Vips []string `yaml:"vipPool"`
}

const (
	VirtualClusterControllerFinalizer = "kosmos.io/virtualcluster-controller"
	RequeueTime                       = 10 * time.Second
)

var nameMap = map[string]int{
	"agentport":  1,
	"serverport": 2,
	"healthport": 3,
	"adminport":  4,
}

func (c *VirtualClusterInitController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing virtual cluster", "virtual cluster", request, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing virtual cluster", "virtual cluster", request, "duration", time.Since(startTime))
	}()

	originalCluster := &v1alpha1.VirtualCluster{}
	if err := c.Get(ctx, request.NamespacedName, originalCluster); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(2).InfoS("Virtual Cluster has been deleted", "Virtual Cluster", request)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{RequeueAfter: RequeueTime}, nil
	}
	updatedCluster := originalCluster.DeepCopy()
	updatedCluster.Status.Reason = ""

	//The object is being deleted
	if !originalCluster.DeletionTimestamp.IsZero() {
		if len(updatedCluster.Spec.PromoteResources.NodeInfos) > 0 {
			updatedCluster.Spec.PromoteResources.NodeInfos = nil
			updatedCluster.Status.Phase = v1alpha1.Deleting
			err := c.Update(updatedCluster)
			if err != nil {
				klog.Errorf("Error update virtualcluster %s status to %s", updatedCluster.Name, updatedCluster.Status.Phase)
				return reconcile.Result{}, errors.Wrapf(err, "Error update virtualcluster %s status", updatedCluster.Name)
			}
			return reconcile.Result{}, nil
		}

		if updatedCluster.Status.Phase == v1alpha1.AllNodeDeleted {
			err := c.destroyVirtualCluster(updatedCluster)
			if err != nil {
				klog.Errorf("Destroy virtual cluter %s failed. err: %s", updatedCluster.Name, err.Error())
				return reconcile.Result{}, errors.Wrapf(err, "Destroy virtual cluter %s failed. err: %s", updatedCluster.Name, err.Error())
			}
			return c.removeFinalizer(updatedCluster)
		} else if updatedCluster.Status.Phase == v1alpha1.Deleting {
			klog.V(2).InfoS("Virtual Cluster is deleting, wait for event 'AllNodeDeleted'", "Virtual Cluster", request)
			return reconcile.Result{}, nil
		}
		return c.removeFinalizer(updatedCluster)
	}

	switch originalCluster.Status.Phase {
	case "":
		//create request
		updatedCluster.Status.Phase = v1alpha1.Preparing
		err := c.Update(updatedCluster)
		if err != nil {
			klog.Errorf("Error update virtualcluster %s status, err: %v", updatedCluster.Name, err)
			return reconcile.Result{RequeueAfter: RequeueTime}, errors.Wrapf(err, "Error update virtualcluster %s status", updatedCluster.Name)
		}

		err = c.createVirtualCluster(updatedCluster, c.KubeNestOptions)
		if err != nil {
			klog.Errorf("Failed to create virtualcluster %s. err: %s", updatedCluster.Name, err.Error())
			updatedCluster.Status.Reason = err.Error()
			updatedCluster.Status.Phase = v1alpha1.Pending
			err := c.Update(updatedCluster)
			if err != nil {
				klog.Errorf("Error update virtualcluster %s. err: %s", updatedCluster.Name, err.Error())
				return reconcile.Result{}, errors.Wrapf(err, "Error update virtualcluster %s status", updatedCluster.Name)
			}
			return reconcile.Result{}, errors.Wrap(err, "Error createVirtualCluster")
		}
		updatedCluster.Status.Phase = v1alpha1.Initialized
		err = c.Update(updatedCluster)
		if err != nil {
			klog.Errorf("Error update virtualcluster %s status to %s. %v", updatedCluster.Name, updatedCluster.Status.Phase, err)
			return reconcile.Result{}, errors.Wrapf(err, "Error update virtualcluster %s status", updatedCluster.Name)
		}
	case v1alpha1.AllNodeReady:
		name, namespace := request.Name, request.Namespace
		// check if the vc enable vip
		if len(originalCluster.Status.VipMap) > 0 {
			// label node for keepalived
			vcClient, err := tasks.GetVcClientset(c.RootClientSet, name, namespace)
			if err != nil {
				klog.Errorf("Get vc client failed. err: %s", err.Error())
				return reconcile.Result{}, errors.Wrapf(err, "Get vc client failed. err: %s", err.Error())
			}
			reps, err := c.labelNode(vcClient)
			if err != nil {
				klog.Errorf("Label node for keepalived failed. err: %s", err.Error())
				return reconcile.Result{}, errors.Wrapf(err, "Label node for keepalived failed. err: %s", err.Error())
			}
			klog.V(2).Infof("Label %d node for keepalived", reps)
		}

		err := c.ensureAllPodsRunning(updatedCluster, constants.WaitAllPodsRunningTimeoutSeconds*time.Second)
		if err != nil {
			klog.Errorf("Check all pods running err: %s", err.Error())
			updatedCluster.Status.Reason = err.Error()
			updatedCluster.Status.Phase = v1alpha1.Pending
		} else {
			updatedCluster.Status.Phase = v1alpha1.Completed
		}
		err = c.Update(updatedCluster)
		if err != nil {
			klog.Errorf("Error update virtualcluster %s status to %s", updatedCluster.Name, updatedCluster.Status.Phase)
			return reconcile.Result{}, errors.Wrapf(err, "Error update virtualcluster %s status", updatedCluster.Name)
		}
	case v1alpha1.Completed:
		//update request, check if promotepolicy nodes increase or decrease.
		// only 2 scenarios matched update request with status 'completed'.
		// 1. node scale request, original status is 'completed'. 2. node scale process finished by NodeController, the controller changes status from 'updating' to 'completed'
		policyChanged, err := c.checkPromotePoliciesChanged(updatedCluster)
		if err != nil {
			klog.Errorf("Error check promote policies changed. err: %s", err.Error())
			return reconcile.Result{RequeueAfter: RequeueTime}, errors.Wrapf(err, "Error checkPromotePoliciesChanged virtualcluster %s", updatedCluster.Name)
		}
		if !policyChanged {
			return reconcile.Result{}, nil
		}
		err = c.assignWorkNodes(updatedCluster)
		if err != nil {
			return reconcile.Result{RequeueAfter: RequeueTime}, errors.Wrapf(err, "Error update virtualcluster %s", updatedCluster.Name)
		}
		updatedCluster.Status.Phase = v1alpha1.Updating
		err = c.Update(updatedCluster)
		if err != nil {
			klog.Errorf("Error update virtualcluster %s status to %s", updatedCluster.Name, updatedCluster.Status.Phase)
			return reconcile.Result{}, errors.Wrapf(err, "Error update virtualcluster %s status", updatedCluster.Name)
		}

	default:
		klog.Warningf("Skip virtualcluster %s reconcile status: %s", originalCluster.Name, originalCluster.Status.Phase)
	}
	return c.ensureFinalizer(updatedCluster)
}

func (c *VirtualClusterInitController) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(constants.InitControllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(&v1alpha1.VirtualCluster{},
			builder.WithPredicates(predicate.Funcs{
				//	UpdateFunc: c.onVirtualClusterUpdate,
				CreateFunc: func(createEvent event.CreateEvent) bool {
					return true
				},
				UpdateFunc: func(updateEvent event.UpdateEvent) bool { return true },
				DeleteFunc: func(deleteEvent event.DeleteEvent) bool { return true },
			})).
		Complete(c)
}

func (c *VirtualClusterInitController) Update(updated *v1alpha1.VirtualCluster) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &v1alpha1.VirtualCluster{}
		if err := c.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: updated.Namespace,
			Name:      updated.Name,
		}, current); err != nil {
			klog.Errorf("get virtualcluster %s error. %v", updated.Name, err)
			return err
		}
		now := metav1.Now()
		updated.Status.UpdateTime = &now
		updated.ResourceVersion = current.ResourceVersion
		return c.Client.Patch(context.TODO(), updated, client.MergeFrom(current))
	})
}

func (c *VirtualClusterInitController) ensureFinalizer(virtualCluster *v1alpha1.VirtualCluster) (reconcile.Result, error) {
	if controllerutil.ContainsFinalizer(virtualCluster, VirtualClusterControllerFinalizer) {
		return reconcile.Result{}, nil
	}
	current := &v1alpha1.VirtualCluster{}
	if err := c.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: virtualCluster.Namespace,
		Name:      virtualCluster.Name,
	}, current); err != nil {
		klog.Errorf("get virtualcluster %s error. %v", virtualCluster.Name, err)
		return reconcile.Result{Requeue: true}, err
	}

	updated := current.DeepCopy()
	controllerutil.AddFinalizer(updated, VirtualClusterControllerFinalizer)
	err := c.Client.Update(context.TODO(), updated)
	if err != nil {
		klog.Errorf("update virtualcluster %s error. %v", virtualCluster.Name, err)
		klog.Errorf("Failed to add finalizer to VirtualCluster %s/%s: %v", virtualCluster.Namespace, virtualCluster.Name, err)
		return reconcile.Result{Requeue: true}, err
	}

	return reconcile.Result{}, nil
}

func (c *VirtualClusterInitController) removeFinalizer(virtualCluster *v1alpha1.VirtualCluster) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(virtualCluster, VirtualClusterControllerFinalizer) {
		return reconcile.Result{}, nil
	}

	current := &v1alpha1.VirtualCluster{}
	if err := c.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: virtualCluster.Namespace,
		Name:      virtualCluster.Name,
	}, current); err != nil {
		klog.Errorf("get virtualcluster %s error. %v", virtualCluster.Name, err)
		return reconcile.Result{Requeue: true}, err
	}
	updated := current.DeepCopy()

	controllerutil.RemoveFinalizer(updated, VirtualClusterControllerFinalizer)
	err := c.Client.Update(context.TODO(), updated)
	if err != nil {
		klog.Errorf("Failed to remove finalizer to VirtualCluster %s/%s: %v", virtualCluster.Namespace, virtualCluster.Name, err)
		return reconcile.Result{Requeue: true}, err
	}

	return reconcile.Result{}, nil
}

// nolint:revive
// createVirtualCluster assign work nodes, create control plane and create compoennts from manifests
func (c *VirtualClusterInitController) createVirtualCluster(virtualCluster *v1alpha1.VirtualCluster, kubeNestOptions *v1alpha1.KubeNestConfiguration) error {
	klog.V(2).Infof("Reconciling virtual cluster", "name", virtualCluster.Name)

	//Assign host port
	_, err := c.AllocateHostPort(virtualCluster, kubeNestOptions)
	if err != nil {
		return errors.Wrap(err, "Error in assign host port!")
	}
	// check if enable vip
	vipPool, err := GetVipFromConfigMap(c.RootClientSet, constants.KosmosNs, constants.VipPoolConfigMapName, constants.VipPoolKey)
	if err == nil && vipPool != nil && len(vipPool.Vips) > 0 {
		klog.V(2).Infof("Enable vip for virtual cluster %s", virtualCluster.Name)
		//Allocate vip
		err = c.AllocateVip(virtualCluster, vipPool)
		if err != nil {
			return errors.Wrap(err, "Error in allocate vip!")
		}
	}

	executer, err := NewExecutor(virtualCluster, c.Client, c.Config, kubeNestOptions)
	if err != nil {
		return err
	}
	err = c.assignWorkNodes(virtualCluster)
	if err != nil {
		return errors.Wrap(err, "Error in assign work nodes")
	}
	klog.V(2).Infof("Successfully assigned work node for virtual cluster %s", virtualCluster.Name)
	getKubeconfig := func() (string, error) {
		secretName := fmt.Sprintf("%s-%s", virtualCluster.GetName(), constants.AdminConfig)
		secret, err := c.RootClientSet.CoreV1().Secrets(virtualCluster.GetNamespace()).Get(context.TODO(), secretName, metav1.GetOptions{})
		if err != nil {
			return "", errors.Wrapf(err, "Failed to get secret %s for virtual cluster %s", secretName, virtualCluster.GetName())
		}
		return base64.StdEncoding.EncodeToString(secret.Data[constants.KubeConfig]), nil
	}
	err = executer.Execute()
	if err != nil {
		virtualCluster.Spec.Kubeconfig, _ = getKubeconfig()
		return err
	}
	virtualCluster.Spec.Kubeconfig, err = getKubeconfig()
	return err
}

func (c *VirtualClusterInitController) destroyVirtualCluster(virtualCluster *v1alpha1.VirtualCluster) error {
	klog.V(2).Infof("Destroying virtual cluster %s", virtualCluster.Name)
	execute, err := NewExecutor(virtualCluster, c.Client, c.Config, c.KubeNestOptions)
	if err != nil {
		return err
	}
	return execute.Execute()
}

func (c *VirtualClusterInitController) assignWorkNodes(virtualCluster *v1alpha1.VirtualCluster) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	globalNodeList, err := c.KosmosClient.KosmosV1alpha1().GlobalNodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list global nodes: %w", err)
	}
	allNodeInfos := make([]v1alpha1.NodeInfo, 0)
	globalNodes := globalNodeList.Items
	sort.Slice(globalNodes, func(i, j int) bool {
		return globalNodes[i].Name < globalNodes[j].Name
	})
	for _, policy := range virtualCluster.Spec.PromotePolicies {
		globalNodes, err := retrieveGlobalNodesWithLabelSelector(globalNodeList.Items, policy.LabelSelector)
		if err != nil {
			return fmt.Errorf("retrieve globalnode with labelselector: %w", err)
		}
		sort.Slice(globalNodes, func(i, j int) bool {
			return globalNodes[i].Name < globalNodes[j].Name
		})
		klog.V(4).Infof("LabelSelected Globalnode count %d", len(globalNodes))
		nodeInfos, err := c.assignNodesByPolicy(virtualCluster, policy, globalNodes)
		if err != nil {
			return fmt.Errorf("assign nodes by policy: %w", err)
		}
		allNodeInfos = append(allNodeInfos, nodeInfos...)
	}

	// set all node status in usage
	for _, nodeInfo := range allNodeInfos {
		globalNode, ok := util.FindGlobalNode(nodeInfo.NodeName, globalNodeList.Items)
		if !ok {
			return fmt.Errorf("assigned node %s doesn't exist in globalnode list. this should not happen normally", nodeInfo.NodeName)
		}

		// only new assigned nodes' status is not `InUse`
		if globalNode.Spec.State != v1alpha1.NodeInUse {
			// Note. Although we tried hard to make sure update globalNode successful in func `setGlobalNodeUsageStatus`.
			//  But in case of failure, some dirty data will occur because of some globalNodes have been marked `InUse`.
			// But virutalcluster's NodeInfos have not been updated yet.
			err = c.setGlobalNodeUsageStatus(virtualCluster, globalNode)
			if err != nil {
				return fmt.Errorf("set globalnode %s InUse error. %v", globalNode.Name, err)
			}

			// Preventive programming. Sometimes promotePolicies may not be well-designed，not absolutely non-overlapping.
			// this may lead to multiple same node in `allNodeInfos`.
			globalNode.Spec.State = v1alpha1.NodeInUse
		}
	}
	virtualCluster.Spec.PromoteResources.NodeInfos = allNodeInfos
	return nil
}

func (c *VirtualClusterInitController) checkPromotePoliciesChanged(virtualCluster *v1alpha1.VirtualCluster) (bool, error) {
	globalNodeList, err := c.KosmosClient.KosmosV1alpha1().GlobalNodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("list global nodes: %w", err)
	}
	for _, policy := range virtualCluster.Spec.PromotePolicies {
		globalNodes, err := retrieveGlobalNodesWithLabelSelector(globalNodeList.Items, policy.LabelSelector)
		if err != nil {
			return false, fmt.Errorf("retrieve globalnode with labelselector: %w", err)
		}
		nodesAssigned, err := retrieveAssignedNodesByPolicy(virtualCluster, globalNodes)
		if err != nil {
			return false, errors.Wrapf(err, "Parse assigned nodes by policy %s error", policy.LabelSelector.String())
		}
		if policy.NodeCount != int32(len(nodesAssigned)) {
			klog.V(2).Infof("Promote policy node count changed from %d to %d", len(nodesAssigned), policy.NodeCount)
			return true, nil
		}
	}
	return false, nil
}

func IsLabelsMatchSelector(selector *metav1.LabelSelector, targetLabels labels.Set) (match bool, err error) {
	if selector == nil {
		return true, nil
	}
	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return false, err
	}

	match = sel.Matches(targetLabels)
	return match, nil
}

// nodesChangeCalculate calculate nodes changed when update virtualcluster.
func (c *VirtualClusterInitController) assignNodesByPolicy(virtualCluster *v1alpha1.VirtualCluster, policy v1alpha1.PromotePolicy, policyMatchedGlobalNodes []v1alpha1.GlobalNode) ([]v1alpha1.NodeInfo, error) {
	nodesAssigned, err := retrieveAssignedNodesByPolicy(virtualCluster, policyMatchedGlobalNodes)
	if err != nil {
		return nil, fmt.Errorf("parse assigned nodes by policy %v error", policy.LabelSelector)
	}

	requestNodesChanged := policy.NodeCount - int32(len(nodesAssigned))
	if requestNodesChanged == 0 {
		klog.V(2).Infof("Nothing to do for policy %v", policy.LabelSelector)
		return nodesAssigned, nil
	} else if requestNodesChanged > 0 {
		// nodes needs to increase
		klog.V(2).Infof("Try allocate %d nodes for policy %v", requestNodesChanged, policy.LabelSelector)
		var newAssignNodesIndex []int
		for i, globalNode := range policyMatchedGlobalNodes {
			if globalNode.Spec.State == v1alpha1.NodeFreeState {
				newAssignNodesIndex = append(newAssignNodesIndex, i)
			}
			if int32(len(newAssignNodesIndex)) == requestNodesChanged {
				break
			}
		}
		if int32(len(newAssignNodesIndex)) < requestNodesChanged {
			return nodesAssigned, errors.Errorf("There is not enough work nodes for promotepolicy %v. Desired %d, matched %d", policy.LabelSelector, requestNodesChanged, len(newAssignNodesIndex))
		}
		for _, index := range newAssignNodesIndex {
			klog.V(2).Infof("Assign node %s for virtualcluster %s policy %v", policyMatchedGlobalNodes[index].Name, virtualCluster.GetName(), policy.LabelSelector)
			nodesAssigned = append(nodesAssigned, v1alpha1.NodeInfo{
				NodeName: policyMatchedGlobalNodes[index].Name,
			})
		}
	} else {
		// nodes needs to decrease
		klog.V(2).Infof("Try decrease nodes %d for policy %v", -requestNodesChanged, policy.LabelSelector)
		decrease := int(-requestNodesChanged)
		if len(nodesAssigned) < decrease {
			return nil, errors.Errorf("Illegal work nodes decrease operation for promotepolicy %v. Desired %d, matched %d", policy.LabelSelector, decrease, len(nodesAssigned))
		}
		nodesAssigned = nodesAssigned[:len(nodesAssigned)-decrease]
		// note: node pool will not be modified here. NodeController will modify it when node delete success
	}
	return nodesAssigned, nil
}

// retrieveAssignedNodesByPolicy retrieve nodes assigned by policy from virtual cluster spec.
// Note: this function only retrieves nodes that match the policy's label selector.
func retrieveAssignedNodesByPolicy(virtualCluster *v1alpha1.VirtualCluster, policyMatchedGlobalNodes []v1alpha1.GlobalNode) ([]v1alpha1.NodeInfo, error) {
	var nodesAssignedMatchedPolicy []v1alpha1.NodeInfo
	for _, nodeInfo := range virtualCluster.Spec.PromoteResources.NodeInfos {
		if _, ok := util.FindGlobalNode(nodeInfo.NodeName, policyMatchedGlobalNodes); ok {
			nodesAssignedMatchedPolicy = append(nodesAssignedMatchedPolicy, nodeInfo)
		}
	}
	return nodesAssignedMatchedPolicy, nil
}

func matchesWithLabelSelector(metaLabels labels.Set, labelSelector *metav1.LabelSelector) (bool, error) {
	if labelSelector == nil {
		return true, nil
	}

	sel, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return false, err
	}

	match := sel.Matches(metaLabels)
	return match, nil
}

func retrieveGlobalNodesWithLabelSelector(nodes []v1alpha1.GlobalNode, labelSelector *metav1.LabelSelector) ([]v1alpha1.GlobalNode, error) {
	matchedNodes := make([]v1alpha1.GlobalNode, 0)
	for _, node := range nodes {
		matched, err := matchesWithLabelSelector(node.Spec.Labels, labelSelector)
		if err != nil {
			return nil, err
		}
		if matched {
			matchedNodes = append(matchedNodes, node)
		}
	}
	return matchedNodes, nil
}

func (c *VirtualClusterInitController) setGlobalNodeUsageStatus(virtualCluster *v1alpha1.VirtualCluster, node *v1alpha1.GlobalNode) error {
	updateSpecFunc := func() error {
		current, err := c.KosmosClient.KosmosV1alpha1().GlobalNodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.Errorf("globalnode %s not found. This should not happen normally", node.Name)
				return nil
			}
			return fmt.Errorf("failed to get globalNode %s: %v", node.Name, err)
		}

		updated := current.DeepCopy()
		updated.Spec.State = v1alpha1.NodeInUse
		_, err = c.KosmosClient.KosmosV1alpha1().GlobalNodes().Update(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			if apierrors.IsConflict(err) {
				return err
			}

			klog.Errorf("failed to update globalNode spec for %s: %v", updated.Name, err)
			return err
		}
		return nil
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, updateSpecFunc); err != nil {
		return err
	}

	updateStatusFunc := func() error {
		current, err := c.KosmosClient.KosmosV1alpha1().GlobalNodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.Errorf("globalnode %s not found. This should not happen normally", node.Name)
				return nil
			}
			return fmt.Errorf("failed to get globalNode %s: %v", node.Name, err)
		}

		updated := current.DeepCopy()
		updated.Status.VirtualCluster = virtualCluster.Name
		_, err = c.KosmosClient.KosmosV1alpha1().GlobalNodes().UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			if apierrors.IsConflict(err) {
				return err
			}

			klog.Errorf("failed to update globalNode status for %s: %v", updated.Name, err)
			return err
		}
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, updateStatusFunc)
}

func (c *VirtualClusterInitController) ensureAllPodsRunning(virtualCluster *v1alpha1.VirtualCluster, timeout time.Duration) error {
	secret, err := c.RootClientSet.CoreV1().Secrets(virtualCluster.GetNamespace()).Get(context.TODO(),
		fmt.Sprintf("%s-%s", virtualCluster.GetName(), constants.AdminConfig), metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Get virtualcluster kubeconfig secret error")
	}
	config, err := clientcmd.RESTConfigFromKubeConfig(secret.Data[constants.KubeConfig])
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	namespaceList, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "List namespaces error")
	}
	endTime := time.Now().Second() + int(timeout.Seconds())
	for _, namespace := range namespaceList.Items {
		startTime := time.Now().Second()
		if startTime > endTime {
			return errors.New("Timeout waiting for all pods running")
		}
		klog.V(2).Infof("Check if all pods ready in namespace %s", namespace.Name)
		err := wait.PollWithContext(context.TODO(), 5*time.Second, time.Duration(endTime-startTime)*time.Second, func(ctx context.Context) (done bool, err error) {
			klog.V(2).Infof("Check if virtualcluster %s all deployments ready in namespace %s", virtualCluster.Name, namespace.Name)
			deployList, err := clientset.AppsV1().Deployments(namespace.Name).List(ctx, metav1.ListOptions{})
			if err != nil {
				return false, errors.Wrapf(err, "Get deployment list in namespace %s error", namespace.Name)
			}
			for _, deploy := range deployList.Items {
				if deploy.Status.AvailableReplicas != deploy.Status.Replicas {
					klog.V(2).Infof("Deployment %s/%s is not ready yet. Available replicas: %d, Desired: %d. Waiting...", deploy.Name, namespace.Name, deploy.Status.AvailableReplicas, deploy.Status.Replicas)
					return false, nil
				}
			}

			klog.V(2).Infof("Check if virtualcluster %s all statefulset ready in namespace %s", virtualCluster.Name, namespace.Name)
			stsList, err := clientset.AppsV1().StatefulSets(namespace.Name).List(ctx, metav1.ListOptions{})
			if err != nil {
				return false, errors.Wrapf(err, "Get statefulset list in namespace %s error", namespace.Name)
			}
			for _, sts := range stsList.Items {
				if sts.Status.AvailableReplicas != sts.Status.Replicas {
					klog.V(2).Infof("Statefulset %s/%s is not ready yet. Available replicas: %d, Desired: %d. Waiting...", sts.Name, namespace.Name, sts.Status.AvailableReplicas, sts.Status.Replicas)
					return false, nil
				}
			}

			klog.V(2).Infof("Check if virtualcluster %s all daemonset ready in namespace %s", virtualCluster.Name, namespace.Name)
			damonsetList, err := clientset.AppsV1().DaemonSets(namespace.Name).List(ctx, metav1.ListOptions{})
			if err != nil {
				return false, errors.Wrapf(err, "Get daemonset list in namespace %s error", namespace.Name)
			}
			for _, daemonset := range damonsetList.Items {
				if daemonset.Status.CurrentNumberScheduled != daemonset.Status.NumberReady {
					klog.V(2).Infof("Daemonset %s/%s is not ready yet. Scheduled replicas: %d, Ready: %d. Waiting...", daemonset.Name, namespace.Name, daemonset.Status.CurrentNumberScheduled, daemonset.Status.NumberReady)
					return false, nil
				}
			}

			return true, nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func GetHostPortPoolFromConfigMap(client kubernetes.Interface, ns, cmName, dataKey string) (*HostPortPool, error) {
	hostPorts, err := client.CoreV1().ConfigMaps(ns).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	yamlData, exist := hostPorts.Data[dataKey]
	if !exist {
		return nil, fmt.Errorf("key '%s' not found in ConfigMap '%s'", dataKey, cmName)
	}

	var hostPool HostPortPool
	if err := yaml.Unmarshal([]byte(yamlData), &hostPool); err != nil {
		return nil, err
	}

	return &hostPool, nil
}

func GetVipFromConfigMap(client kubernetes.Interface, ns, cmName, key string) (*VipPool, error) {
	vipPoolCm, err := client.CoreV1().ConfigMaps(ns).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	yamlData, exist := vipPoolCm.Data[key]
	if !exist {
		return nil, fmt.Errorf("key '%s' not found in vip pool ConfigMap '%s'", key, cmName)
	}

	var vipPool VipPool
	if err := yaml.Unmarshal([]byte(yamlData), &vipPool); err != nil {
		return nil, err
	}

	return &vipPool, nil
}

// Return false to indicate that the port is not occupied
func (c *VirtualClusterInitController) isPortAllocated(port int32, hostAddress []string) bool {
	vcList := &v1alpha1.VirtualClusterList{}
	err := c.List(context.Background(), vcList)
	if err != nil {
		klog.Errorf("list virtual cluster error: %v", err)
		return true
	}

	for _, vc := range vcList.Items {
		// 判断一个map是否包含某个端口
		contains := func(port int32) bool {
			for _, p := range vc.Status.PortMap {
				if p == port {
					return true
				}
			}
			return false
		}
		if vc.Status.Port == port || contains(port) {
			return true
		}
	}

	ret, err := checkPortOnHostWithAddresses(port, hostAddress)
	if err != nil {
		klog.Errorf("check port on host error: %v", err)
		return true
	}
	return ret
}

// Return false to indicate that the port is not occupied
func checkPortOnHostWithAddresses(port int32, hostAddress []string) (bool, error) {
	for _, addr := range hostAddress {
		flag, err := CheckPortOnHost(addr, port)
		if err != nil {
			return false, err
		}
		if flag {
			return true, nil
		}
	}
	return false, nil
}

// Return false to indicate that the port is not occupied
func CheckPortOnHost(addr string, port int32) (bool, error) {
	hostExectorHelper := exector.NewExectorHelper(addr, "")
	checkCmd := &exector.CheckExector{
		Port: fmt.Sprintf("%d", port),
	}

	var ret *exector.ExectorReturn
	err := apiclient.TryRunCommand(func() error {
		ret = hostExectorHelper.DoExector(context.TODO().Done(), checkCmd)
		if ret.Code != 1000 {
			return fmt.Errorf("chekc port failed, err: %s", ret.String())
		}
		return nil
	}, 3)

	if err != nil {
		klog.Errorf("check port on host error! addr:%s, port %d, err: %s", addr, port, err.Error())
		return true, err
	}

	if ret.Status != exector.SUCCESS {
		return true, fmt.Errorf("pod[%d] is occupied", port)
	}
	return false, nil
}

func (c *VirtualClusterInitController) findHostAddresses() ([]string, error) {
	nodes, err := c.RootClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: env.GetControlPlaneLabel(),
	})
	if err != nil {
		return nil, err
	}

	ret := []string{}

	for _, node := range nodes.Items {
		addr, err := utils.FindFirstNodeIPAddress(node, constants.PreferredAddressType)
		if err != nil {
			return nil, err
		}

		ret = append(ret, addr)
	}
	return ret, nil
}

func (c *VirtualClusterInitController) GetHostPortNextFunc(_ *v1alpha1.VirtualCluster) (func() (int32, error), error) {
	var hostPool *HostPortPool
	var err error
	type nextfunc func() (int32, error)
	var next nextfunc
	hostPool, err = GetHostPortPoolFromConfigMap(c.RootClientSet, constants.KosmosNs, constants.HostPortsCMName, constants.HostPortsCMDataName)
	if err != nil {
		klog.Errorf("get host port pool error: %v", err)
		return nil, err
	}
	next = func() nextfunc {
		i := 0
		return func() (int32, error) {
			if i >= len(hostPool.PortsPool) {
				return 0, fmt.Errorf("no available ports")
			}
			port := hostPool.PortsPool[i]
			i++
			return port, nil
		}
	}()
	// }
	return next, nil
}

func createAPIAnpAgentSvc(name, namespace string, nameMap map[string]int) *corev1.Service {
	apiAnpAgentSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.GetKonnectivityAPIServerName(name),
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: func() []corev1.ServicePort {
				ret := []corev1.ServicePort{}
				for k, v := range nameMap {
					ret = append(ret, corev1.ServicePort{
						Port:     8080 + int32(v),
						Protocol: corev1.ProtocolTCP,
						TargetPort: intstr.IntOrString{
							IntVal: 8080 + int32(v),
						},
						Name: k,
					})
				}
				return ret
			}(),
		},
	}
	return apiAnpAgentSvc
}

func (c *VirtualClusterInitController) GetNodePorts(client kubernetes.Interface, virtualCluster *v1alpha1.VirtualCluster) ([]int32, error) {
	ports := make([]int32, 5)
	ipFamilies := utils.IPFamilyGenerator(constants.APIServerServiceSubnet)
	name := virtualCluster.GetName()
	namespace := virtualCluster.GetNamespace()
	apiSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.GetAPIServerName(name),
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Port:     30007, // just for get node port
					Protocol: corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						IntVal: 8080, // just for get node port
					},
					Name: "client",
				},
			},
			IPFamilies: ipFamilies,
		},
	}
	err := util.CreateOrUpdateService(client, apiSvc)
	if err != nil {
		return nil, fmt.Errorf("can not create api svc for allocate port, error: %s", err)
	}

	createdAPISvc, err := client.CoreV1().Services(namespace).Get(context.TODO(), apiSvc.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can not get api svc for allocate port, error: %s", err)
	}
	nodePort := createdAPISvc.Spec.Ports[0].NodePort
	ports[0] = nodePort

	apiAnpAgentSvc := createAPIAnpAgentSvc(name, namespace, nameMap)
	err = util.CreateOrUpdateService(client, apiAnpAgentSvc)
	if err != nil {
		return nil, fmt.Errorf("can not create anp svc for allocate port, error: %s", err)
	}

	createdAnpSvc, err := client.CoreV1().Services(namespace).Get(context.TODO(), apiAnpAgentSvc.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can not get api svc for allocate port, error: %s", err)
	}

	for _, port := range createdAnpSvc.Spec.Ports {
		v, ok := nameMap[port.Name]
		if ok {
			ports[v] = port.NodePort
		} else {
			return nil, fmt.Errorf("can not get node port for %s", port.Name)
		}
	}

	return ports, nil
}

func (c *VirtualClusterInitController) GetHostNetworkPorts(virtualCluster *v1alpha1.VirtualCluster) ([]int32, error) {
	next, err := c.GetHostPortNextFunc(virtualCluster)
	if err != nil {
		return nil, err
	}

	hostAddress, err := c.findHostAddresses()
	if err != nil {
		return nil, err
	}

	// 检查是否手动指定了 APIServerPortKey 的端口号
	var specifiedAPIServerPort int32
	if virtualCluster.Spec.KubeInKubeConfig != nil && virtualCluster.Spec.KubeInKubeConfig.ExternalPort != 0 {
		specifiedAPIServerPort = virtualCluster.Spec.KubeInKubeConfig.ExternalPort
		klog.V(4).InfoS("APIServerPortKey specified manually", "port", specifiedAPIServerPort)
	}

	// 保存最终的分配结果
	ports := make([]int32, 0)

	// 如果手动指定了 APIServerPortKey 的端口，先检查端口是否可用
	if specifiedAPIServerPort != 0 {
		// 检查手动指定的端口是否已经被占用
		if !c.isPortAllocated(specifiedAPIServerPort, hostAddress) {
			ports = append(ports, specifiedAPIServerPort) // 使用手动指定的端口
		} else {
			// 如果指定的端口已经被占用，则返回错误
			klog.Errorf("Specified APIServerPortKey port %d is already allocated", specifiedAPIServerPort)
			return nil, fmt.Errorf("specified APIServerPortKey port %d is already allocated", specifiedAPIServerPort)
		}
	}

	// 从端口池中继续分配剩余的端口（确保端口数量满足要求）
	for p, err := next(); err == nil; p, err = next() {
		// 检查生成的端口是否被占用
		if !c.isPortAllocated(p, hostAddress) {
			ports = append(ports, p)
			if len(ports) >= constants.VirtualClusterPortNum {
				break // 分配到足够的端口后退出
			}
		}
	}

	// 检查分配的端口数量是否足够
	if len(ports) < constants.VirtualClusterPortNum {
		klog.Errorf("No available ports to allocate, need %d, got %d", constants.VirtualClusterPortNum, len(ports))
		return nil, fmt.Errorf("no available ports to allocate, need %d, got %d", constants.VirtualClusterPortNum, len(ports))
	}

	return ports, nil
}

// AllocateHostPort allocate host port for virtual cluster
// #nosec G602
func (c *VirtualClusterInitController) AllocateHostPort(virtualCluster *v1alpha1.VirtualCluster, _ *v1alpha1.KubeNestConfiguration) (int32, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if len(virtualCluster.Status.PortMap) > 0 || virtualCluster.Status.Port != 0 {
		return 0, nil
	}

	var ports []int32
	var err error

	if virtualCluster.Spec.KubeInKubeConfig != nil && virtualCluster.Spec.KubeInKubeConfig.APIServerServiceType == v1alpha1.NodePort {
		ports, err = c.GetNodePorts(c.RootClientSet, virtualCluster)
	} else {
		ports, err = c.GetHostNetworkPorts(virtualCluster)
	}

	if err != nil {
		return 0, err
	}

	if len(ports) < constants.VirtualClusterPortNum {
		klog.Errorf("no available ports to allocate")
		return 0, fmt.Errorf("no available ports to allocate")
	}
	virtualCluster.Status.PortMap = make(map[string]int32)
	virtualCluster.Status.PortMap[constants.APIServerPortKey] = ports[0]
	virtualCluster.Status.PortMap[constants.APIServerNetworkProxyAgentPortKey] = ports[1]
	virtualCluster.Status.PortMap[constants.APIServerNetworkProxyServerPortKey] = ports[2]
	virtualCluster.Status.PortMap[constants.APIServerNetworkProxyHealthPortKey] = ports[3]
	virtualCluster.Status.PortMap[constants.APIServerNetworkProxyAdminPortKey] = ports[4]

	klog.V(4).InfoS("Success allocate virtual cluster ports", "allocate ports", ports, "vc ports", ports[:2])

	return 0, err
}

// AllocateVip allocate vip for virtual cluster
// nolint:revive
// #nosec G602
func (c *VirtualClusterInitController) AllocateVip(virtualCluster *v1alpha1.VirtualCluster, vipPool *VipPool) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if len(virtualCluster.Status.VipMap) > 0 {
		return nil
	}
	klog.V(4).InfoS("get vip pool", "vipPool", vipPool)
	externalVips := virtualCluster.Spec.KubeInKubeConfig.TenantEntrypoint.ExternalVips
	// check if specified vip is available
	if len(externalVips) > 0 {
		if ip, err := util.IsIPAvailable(externalVips, vipPool.Vips); err != nil {
			klog.Errorf("check if specified vip is available error: %v", err)
			return err
		} else {
			klog.V(4).InfoS("specified vip is available", "vip", ip)
			virtualCluster.Status.VipMap = make(map[string]string)
			virtualCluster.Status.VipMap[constants.VcVipStatusKey] = ip
			return nil
		}
	}
	vcList := &v1alpha1.VirtualClusterList{}
	err := c.List(context.Background(), vcList)
	if err != nil {
		klog.Errorf("list virtual cluster error: %v", err)
		return err
	}
	var allocatedVips []string
	for _, vc := range vcList.Items {
		for _, val := range vc.Status.VipMap {
			allocatedVips = append(allocatedVips, val)
		}
	}

	vip, err := util.FindAvailableIP(vipPool.Vips, allocatedVips)
	if err != nil {
		klog.Errorf("find available vip error: %v", err)
		return err
	}
	virtualCluster.Status.VipMap = make(map[string]string)
	virtualCluster.Status.VipMap[constants.VcVipStatusKey] = vip

	return err
}

func (c *VirtualClusterInitController) labelNode(client kubernetes.Interface) (reps int, err error) {
	replicas := constants.VipKeepAlivedReplicas
	nodes, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list nodes, err: %w", err)
	}
	if len(nodes.Items) == 0 {
		return 0, fmt.Errorf("no nodes found")
	}
	reps = replicas
	// select replicas nodes
	if replicas > len(nodes.Items) {
		reps = len(nodes.Items)
	}
	randomIndex, err := util.SecureRandomInt(reps)
	if err != nil {
		klog.Errorf("failed to get random index for master node, err: %v", err)
		return 0, err
	}
	// sub reps as nodes
	subNodes := nodes.Items[:reps]
	masterNode := nodes.Items[randomIndex]

	// label node
	for _, node := range subNodes {
		currentNode := node
		labels := currentNode.GetLabels()
		if currentNode.Name == masterNode.Name {
			// label master
			labels[constants.VipKeepAlivedNodeRoleKey] = constants.VipKeepAlivedNodeRoleMaster
		} else {
			// label backup
			labels[constants.VipKeepAlivedNodeRoleKey] = constants.VipKeepalivedNodeRoleBackup
		}
		labels[constants.VipKeepAlivedNodeLabelKey] = constants.VipKeepAlivedNodeLabelValue

		// update label
		currentNode.SetLabels(labels)
		_, err := client.CoreV1().Nodes().Update(context.TODO(), &currentNode, metav1.UpdateOptions{})
		if err != nil {
			klog.V(2).Infof("Failed to update labels for node %s: %v", currentNode.Name, err)
			return 0, err
		}
		klog.V(2).Infof("Successfully updated labels for node %s", currentNode.Name)
	}
	klog.V(2).InfoS("[vip] Successfully label all node")
	return reps, nil
}
