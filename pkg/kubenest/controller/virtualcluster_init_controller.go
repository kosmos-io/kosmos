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
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	apiclient "github.com/kosmos.io/kosmos/pkg/kubenest/util/api-client"
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

const (
	VirtualClusterControllerFinalizer = "kosmos.io/virtualcluster-controller"
	RequeueTime                       = 10 * time.Second
)

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
		} else {
			return c.removeFinalizer(updatedCluster)
		}
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
		} else {
			err := c.assignWorkNodes(updatedCluster)
			if err != nil {
				return reconcile.Result{RequeueAfter: RequeueTime}, errors.Wrapf(err, "Error update virtualcluster %s", updatedCluster.Name)
			}
			updatedCluster.Status.Phase = v1alpha1.Updating
			err = c.Update(updatedCluster)
			if err != nil {
				klog.Errorf("Error update virtualcluster %s status to %s", updatedCluster.Name, updatedCluster.Status.Phase)
				return reconcile.Result{}, errors.Wrapf(err, "Error update virtualcluster %s status", updatedCluster.Name)
			}
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
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
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
	}); err != nil {
		return err
	}
	return nil
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

// createVirtualCluster assign work nodes, create control plane and create compoennts from manifests
func (c *VirtualClusterInitController) createVirtualCluster(virtualCluster *v1alpha1.VirtualCluster, kubeNestOptions *v1alpha1.KubeNestConfiguration) error {
	klog.V(2).Infof("Reconciling virtual cluster", "name", virtualCluster.Name)

	//Assign host port
	_, err := c.AllocateHostPort(virtualCluster)
	if err != nil {
		return errors.Wrap(err, "Error in assign host port!")
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
				// 如果节点不存在，则不执行更新并返回nil
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

	if err := retry.RetryOnConflict(retry.DefaultRetry, updateStatusFunc); err != nil {
		return err
	}
	return nil
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

func findAddress(node corev1.Node) (string, error) {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address, nil
		}
	}
	return "", fmt.Errorf("cannot find internal IP address in node addresses, node name: %s", node.GetName())
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
	} else {
		return false, nil
	}
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
		addr, err := findAddress(node)
		if err != nil {
			return nil, err
		}

		ret = append(ret, addr)
	}
	return ret, nil
}

// AllocateHostPort allocate host port for virtual cluster
// #nosec G602
func (c *VirtualClusterInitController) AllocateHostPort(virtualCluster *v1alpha1.VirtualCluster) (int32, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if len(virtualCluster.Status.PortMap) > 0 || virtualCluster.Status.Port != 0 {
		return 0, nil
	}
	// 获取主机端口池
	hostPool, err := GetHostPortPoolFromConfigMap(c.RootClientSet, constants.KosmosNs, constants.HostPortsCMName, constants.HostPortsCMDataName)
	if err != nil {
		return 0, err
	}
	isPortInPool := func(port int32) bool {
		for _, p := range hostPool.PortsPool {
			if p == port {
				return true
			}
		}
		return false
	}

	if virtualCluster.Spec.ExternalPort != 0 && !isPortInPool(virtualCluster.Spec.ExternalPort) {
		return 0, fmt.Errorf("ExternalPort is not in host pool")
	}

	hostAddress, err := c.findHostAddresses()
	if err != nil {
		return 0, err
	}

	// 准备端口分配列表
	ports := func() []int32 {
		ports := make([]int32, 0)
		if virtualCluster.Spec.ExternalPort != 0 && !c.isPortAllocated(virtualCluster.Spec.ExternalPort, hostAddress) {
			ports = append(ports, virtualCluster.Spec.ExternalPort)
		} else if virtualCluster.Spec.ExternalPort != 0 && c.isPortAllocated(virtualCluster.Spec.ExternalPort, hostAddress) {
			return nil
		}
		for _, p := range hostPool.PortsPool {
			if !c.isPortAllocated(p, hostAddress) {
				ports = append(ports, p)
				if len(ports) > constants.VirtualClusterPortNum {
					break
				}
			}
		}
		return ports
	}()
	if ports == nil {
		return 0, fmt.Errorf("port is already being used")
	}
	//可分配端口不够
	if len(ports) < constants.VirtualClusterPortNum {
		return 0, fmt.Errorf("no available ports to allocate")
	}
	// 分配端口并更新 PortMap
	virtualCluster.Status.PortMap = make(map[string]int32)
	virtualCluster.Status.PortMap[constants.ApiServerPortKey] = ports[0]
	virtualCluster.Status.PortMap[constants.ApiServerNetworkProxyAgentPortKey] = ports[1]
	virtualCluster.Status.PortMap[constants.ApiServerNetworkProxyServerPortKey] = ports[2]
	virtualCluster.Status.PortMap[constants.ApiServerNetworkProxyHealthPortKey] = ports[3]
	virtualCluster.Status.PortMap[constants.ApiServerNetworkProxyAdminPortKey] = ports[4]

	klog.V(4).InfoS("Success allocate virtual cluster ports", "allocate ports", ports, "vc ports", ports[:2])

	return 0, err
}
