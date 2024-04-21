package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	vcnodecontroller "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller"
)

type VirtualClusterInitController struct {
	client.Client
	Config          *rest.Config
	EventRecorder   record.EventRecorder
	HostPortManager *vcnodecontroller.HostPortManager
	RootClientSet   kubernetes.Interface
}

var lock sync.Mutex

type NodePool struct {
	Address string            `json:"address" yaml:"address"`
	Labels  map[string]string `json:"labels" yaml:"labels"`
	Cluster string            `json:"cluster" yaml:"cluster"`
	State   string            `json:"state" yaml:"state"`
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
		return reconcile.Result{}, nil
	}
	updatedCluster := originalCluster.DeepCopy()

	//The object is being deleted
	if !originalCluster.DeletionTimestamp.IsZero() {
		//TODO
		return reconcile.Result{}, nil
	}

	if originalCluster.Status.Phase == "" { //create request
		updatedCluster.Status.Phase = v1alpha1.Preparing
		err := c.Client.Patch(context.TODO(), updatedCluster, client.MergeFrom(originalCluster))
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "Error update virtualcluster %s status", updatedCluster.Name)
		}

		err = c.syncVirtualCluster(updatedCluster)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, "Error syncVirtualCluster")
		}
		updatedCluster.Status.Phase = v1alpha1.Initialized
		err = c.Client.Patch(context.TODO(), updatedCluster, client.MergeFrom(originalCluster))
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "Error update virtualcluster %s status", updatedCluster.Name)
		}
	} else if originalCluster.Status.Phase == v1alpha1.AllNodeReady {
		err := c.ensureAllPodsRunning(updatedCluster, constants.WaitAllPodsRunningTimeoutSeconds*time.Second)
		if err != nil {
			updatedCluster.Status.Phase = v1alpha1.Pending
		} else {
			updatedCluster.Status.Phase = v1alpha1.Completed
		}
		err = c.Client.Patch(context.TODO(), updatedCluster, client.MergeFrom(originalCluster))
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "Error update virtualcluster %s status", updatedCluster.Name)
		}
	} else if originalCluster.Status.Phase == v1alpha1.Completed {
		//update request, check if promotepolicy nodes increase or decrease.
		assigned, err := c.assignWorkNodes(updatedCluster)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "Error update virtualcluster %s", updatedCluster.Name)
		}
		if assigned { // indicate nodes change request
			updatedCluster.Status.Phase = v1alpha1.Updating
			err = c.Client.Patch(context.TODO(), updatedCluster, client.MergeFrom(originalCluster))
			if err != nil {
				return reconcile.Result{}, errors.Wrapf(err, "Error update virtualcluster %s status", updatedCluster.Name)
			}
		}
	}
	return reconcile.Result{}, nil
}

func (c *VirtualClusterInitController) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(constants.InitControllerName).
		WithOptions(controller.Options{}).
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

// syncVirtualCluster assign work nodes, create control plane and create compoennts from manifests
func (c *VirtualClusterInitController) syncVirtualCluster(virtualCluster *v1alpha1.VirtualCluster) error {
	klog.V(2).Infof("Reconciling virtual cluster", "name", virtualCluster.Name)

	executer, err := NewExecutor(virtualCluster, c.Client, c.Config, c.HostPortManager)
	if err != nil {
		return err
	}
	_, err = c.assignWorkNodes(virtualCluster)
	if err != nil {
		return errors.Wrap(err, "Error in assign work nodes")
	}
	klog.V(2).Infof("Successfully assigned work node for virtual cluster %s", virtualCluster.Name)
	err = executer.Execute()
	if err != nil {
		return err
	}

	secret, err := c.RootClientSet.CoreV1().Secrets(virtualCluster.GetNamespace()).Get(context.TODO(),
		fmt.Sprintf("%s-%s", virtualCluster.GetName(), constants.AdminConfig), metav1.GetOptions{})
	if err != nil {
		return err
	}
	virtualCluster.Spec.Kubeconfig = base64.StdEncoding.EncodeToString(secret.Data[constants.KubeConfig])
	virtualCluster.Status.Phase = v1alpha1.Completed
	return nil
}

// assignWorkNodes assign nodes for virtualcluster when creating or updating. return true if successfully assigned
func (c *VirtualClusterInitController) assignWorkNodes(virtualCluster *v1alpha1.VirtualCluster) (bool, error) {
	promotepolicies := virtualCluster.Spec.PromotePolicies
	if len(promotepolicies) == 0 {
		return false, errors.New("PromotePolicies parameter undefined")
	}
	lock.Lock()
	defer lock.Unlock()
	nodePool, err := c.getNodePool()
	klog.V(2).Infof("Get node pool %v", nodePool)
	if err != nil {
		return false, errors.Wrap(err, "Get node pool error.")
	}
	klog.V(2).Infof("Total %d nodes in pool", len(nodePool))
	assigned := false
	for _, policy := range promotepolicies {
		assignedByPolicy, nodeInfos, err := c.assignNodesByPolicy(virtualCluster, policy, nodePool)
		if err != nil {
			return false, errors.Wrap(err, "Reassign nodes error")
		}
		if !assignedByPolicy {
			continue
		} else {
			assigned = true
			virtualCluster.Spec.PromoteResources.NodeInfos = nodeInfos
		}
	}
	if assigned {
		err := c.updateNodePool(nodePool)
		if err != nil {
			return false, errors.Wrap(err, "Update node pool error.")
		}
	}
	return assigned, nil
}

// getNodePool get node pool configmap
func (c *VirtualClusterInitController) getNodePool() (map[string]NodePool, error) {
	nodesPoolCm, err := c.RootClientSet.CoreV1().ConfigMaps(constants.KosmosNs).Get(context.TODO(), constants.NodePoolConfigmap, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	var nodesPool map[string]NodePool
	data, ok := nodesPoolCm.Data["nodes"]
	if !ok {
		return nil, errors.New("Error parse nodes pool data")
	}
	err = yaml.Unmarshal([]byte(data), &nodesPool)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshal nodes pool data error")
	}
	return nodesPool, nil
}

// updateNodePool update node pool configmap
func (c *VirtualClusterInitController) updateNodePool(nodePool map[string]NodePool) error {
	klog.V(2).Infof("Update node pool %v", nodePool)
	nodePoolYAML, err := yaml.Marshal(nodePool)
	if err != nil {
		return errors.Wrap(err, "Serialized node pool data error")
	}

	originalCm, err := c.RootClientSet.CoreV1().ConfigMaps(constants.KosmosNs).Get(context.TODO(), constants.NodePoolConfigmap, metav1.GetOptions{})
	if err != nil {
		return err
	}
	originalCm.Data = map[string]string{
		"nodes": string(nodePoolYAML),
	}

	_, err = c.RootClientSet.CoreV1().ConfigMaps(constants.KosmosNs).Update(context.TODO(), originalCm, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "Update node pool configmap data error")
	}
	klog.V(2).Info("Update node pool Success")
	return nil
}

// nodesChangeCalculate calculate nodes changed when update virtualcluster.
func (c *VirtualClusterInitController) assignNodesByPolicy(virtualCluster *v1alpha1.VirtualCluster, policy v1alpha1.PromotePolicy, nodesPool map[string]NodePool) (bool, []v1alpha1.NodeInfo, error) {
	var matched int32 = 0
	var nodesAssignedMatchedPolicy []v1alpha1.NodeInfo
	var nodesAssignedUnMatched []v1alpha1.NodeInfo
	nodesAssigned := virtualCluster.Spec.PromoteResources.NodeInfos
	for _, nodeInfo := range nodesAssigned {
		node, ok := nodesPool[nodeInfo.NodeName]
		if !ok {
			return false, nodesAssigned, errors.Errorf("Node %s doesn't find in nodes pool", nodeInfo.NodeName)
		}
		if mapContains(node.Labels, policy.LabelSelector.MatchLabels) {
			nodesAssignedMatchedPolicy = append(nodesAssignedMatchedPolicy, nodeInfo)
			matched++
		} else {
			nodesAssignedUnMatched = append(nodesAssignedUnMatched, nodeInfo)
		}
	}
	requestNodesChanged := *policy.NodeCount - matched
	if requestNodesChanged == 0 {
		klog.V(2).Infof("Nothing to do for policy %s", policy.LabelSelector.String())
		return false, nodesAssigned, nil
	} else if requestNodesChanged > 0 { // nodes needs to be increased
		klog.V(2).Infof("Try allocate %d nodes for policy %s", requestNodesChanged, policy.LabelSelector.String())
		var cnt int32 = 0
		for name, nodeInfo := range nodesPool {
			if nodeInfo.State == constants.NodeFreeState && mapContains(nodeInfo.Labels, policy.LabelSelector.MatchLabels) {
				nodeInfo.State = constants.NodeVirtualclusterState
				nodeInfo.Cluster = virtualCluster.Name
				nodesPool[name] = nodeInfo
				nodesAssigned = append(nodesAssigned, v1alpha1.NodeInfo{
					NodeName: name,
				})
				cnt++
			}
			if cnt == requestNodesChanged {
				break
			}
		}
		if cnt < requestNodesChanged {
			return false, nodesAssigned, errors.Errorf("There is not enough work nodes for promotepolicy %s. Desired %d, matched %d", policy.LabelSelector.String(), requestNodesChanged, matched)
		}
	} else { // nodes needs to decrease
		klog.V(2).Infof("Try decrease nodes %d for policy %s", -requestNodesChanged, policy.LabelSelector.String())
		decrease := int(-requestNodesChanged)
		if len(nodesAssignedMatchedPolicy) < decrease {
			return false, nodesAssigned, errors.Errorf("Illegal work nodes decrease operation for promotepolicy %s. Desired %d, matched %d", policy.LabelSelector.String(), decrease, len(nodesAssignedMatchedPolicy))
		}
		nodesAssigned = append(nodesAssignedUnMatched, nodesAssignedMatchedPolicy[:(len(nodesAssignedMatchedPolicy)-decrease)]...)
		// note: node pool will not be modified here. NodeController will modify it when node delete success
	}
	return true, nodesAssigned, nil
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
		err := wait.PollWithContext(context.TODO(), 5*time.Second, time.Duration(endTime-startTime)*time.Second, func(ctx context.Context) (done bool, err error) {
			podList, err := clientset.CoreV1().Pods(namespace.Name).List(ctx, metav1.ListOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			if err != nil {
				return true, errors.Wrap(err, "List pods error")
			}

			for _, pod := range podList.Items {
				if pod.Status.Phase != v1.PodRunning {
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

func mapContains(big map[string]string, small map[string]string) bool {
	for k, v := range small {
		if bigV, ok := big[k]; !ok || bigV != v {
			return false
		}
	}
	return true
}
