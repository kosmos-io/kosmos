package clusterManager

import (
	"context"
	"fmt"
	"sync"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"k8s.io/utils/pointer"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/options"
	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	podcontrollers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod"
	leafpodsyncers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod/leaf-pod"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	NodeLeaseControllerName = "node-lease-controller"

	DefaultLeaseDuration         = 40
	DefaultRenewIntervalFraction = 0.25

	DefaultNodeStatusUpdateInterval = 1 * time.Minute
)

func CreateOpenApiNode(ctx context.Context, cluster *kosmosv1alpha1.Cluster, rootClientset kubernetes.Interface, opts *options.Options) error {
	// create node
	nodeNameInRoot := fmt.Sprintf("%s%s", utils.KosmosNodePrefix, cluster.Name)
	nodeInRoot, err := createNode(ctx, rootClientset, cluster.Name, nodeNameInRoot, "v1.21.5-eki.0", opts.ListenPort)
	if err != nil {
		return err
	}

	nodes := []*corev1.Node{nodeInRoot}
	// lease / resources
	nodelease := NewNodeLeaseController(nodes, rootClientset)

	go func() {
		if err := nodelease.Start(ctx); err != nil {
			klog.Fatal(err)
		}
	}()

	// pod
	if cluster.Spec.ClusterTreeOptions == nil {
		return fmt.Errorf("clusterTreeOptions is nil")
	}
	ak := cluster.Spec.ClusterTreeOptions.AccessKey
	sk := cluster.Spec.ClusterTreeOptions.SecretKey

	if len(ak) == 0 || len(sk) == 0 {
		return fmt.Errorf("ak/sk is nil")
	}

	// TODO: modify CRD
	serverlessClient := leafUtils.NewServerlessClient(ak, sk, "CIDC-RP-25", "http://10.253.26.218:18080")
	leafUtils.GetGlobalLeafResourceManager().AddLeafResource(&leafUtils.LeafResource{
		LeafType:         leafUtils.LeafTypeServerless,
		ServerlessClient: serverlessClient,
	}, cluster, nodes)

	leafPodWorkerQueue := podcontrollers.NewLeafPodWorkerQueue(&leafpodsyncers.LeafPodWorkerQueueOption{
		// Config:     leafRestConfig,
		RootClient:       rootClientset,
		ServerlessClient: serverlessClient,
	}, leafUtils.LeafTypeServerless) // TODO:

	go leafPodWorkerQueue.Run(ctx)

	return nil
}

func createNode(ctx context.Context, clientset kubernetes.Interface, clusterName, nodeName, gitVersion string, listenPort int32) (*corev1.Node, error) {
	nodeInRoot, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		nodeInRoot = utils.BuildNodeTemplate(nodeName)
		nodeAnnotations := nodeInRoot.GetAnnotations()
		if nodeAnnotations == nil {
			nodeAnnotations = make(map[string]string, 1)
		}
		nodeAnnotations[utils.KosmosNodeOwnedByClusterAnnotations] = clusterName
		nodeInRoot.SetAnnotations(nodeAnnotations)

		nodeInRoot.Status.NodeInfo.KubeletVersion = gitVersion
		nodeInRoot.Status.DaemonEndpoints = corev1.NodeDaemonEndpoints{
			KubeletEndpoint: corev1.DaemonEndpoint{
				Port: listenPort,
			},
		}

		nodeInRoot, err = clientset.CoreV1().Nodes().Create(ctx, nodeInRoot, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
	}
	return nodeInRoot, nil
}

func NewNodeLeaseController(nodes []*corev1.Node, rootClient kubernetes.Interface) *NodeLeaseController {
	c := &NodeLeaseController{
		rootClient:     rootClient,
		nodes:          nodes,
		leaseInterval:  getRenewInterval(),
		statusInterval: DefaultNodeStatusUpdateInterval,
	}
	return c
}

type NodeLeaseController struct {
	nodes          []*corev1.Node
	nodeLock       sync.Mutex
	rootClient     kubernetes.Interface
	leaseInterval  time.Duration
	statusInterval time.Duration
}

func (c *NodeLeaseController) Start(ctx context.Context) error {
	go wait.UntilWithContext(ctx, c.syncLease, c.leaseInterval)
	go wait.UntilWithContext(ctx, c.syncNodeStatus, c.statusInterval)
	<-ctx.Done()
	return nil
}

func (c *NodeLeaseController) syncNodeStatus(ctx context.Context) {
	nodes := make([]*corev1.Node, 0)
	c.nodeLock.Lock()
	for _, nodeIndex := range c.nodes {
		nodeCopy := nodeIndex.DeepCopy()
		nodes = append(nodes, nodeCopy)
	}
	c.nodeLock.Unlock()

	err := c.updateNodeStatus(ctx, nodes)
	if err != nil {
		klog.Errorf(err.Error())
	}
}

func (c *NodeLeaseController) updateNodeStatus(ctx context.Context, n []*corev1.Node) error {
	// get the node from root cluster as a copy node
	rootnodes, err := c.rootClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		// get the node which is not created by kosmos
		LabelSelector: fmt.Sprintf("%s!=%s", utils.KosmosNodeLabel, utils.KosmosNodeValue),
	})
	if err != nil {
		return fmt.Errorf("create node failed, cannot get node from root cluster, err: %v", err)
	}

	if len(rootnodes.Items) == 0 {
		return fmt.Errorf("create node failed, cannot get node from root cluster, len of leafnodes is 0")
	}

	copynode := rootnodes.Items[0]

	for _, node := range n {
		node.Status = copynode.DeepCopy().Status
		// remove address
		node.Status.Addresses = []corev1.NodeAddress{}
		_, err := c.rootClient.CoreV1().Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("Could not update node status in root cluster,Error: %v", err)
		}
	}
	return nil
}

func (c *NodeLeaseController) syncLease(ctx context.Context) {
	nodes := make([]*corev1.Node, 0)
	c.nodeLock.Lock()
	for _, nodeIndex := range c.nodes {
		nodeCopy := nodeIndex.DeepCopy()
		nodes = append(nodes, nodeCopy)
	}
	c.nodeLock.Unlock()

	// TODO： ping openapi
	// _, err := c.leafClient.Discovery().ServerVersion()
	// if err != nil {
	// 	klog.Errorf("failed to ping leaf cluster")
	// 	return
	// }

	err := c.createLeaseIfNotExists(ctx, nodes)
	if err != nil {
		return
	}

	err = c.updateLeaseWithRetry(ctx, nodes)
	if err != nil {
		klog.Errorf("lease has failed, and the maximum number of retries has been reached, %v", err)
		return
	}

	klog.V(5).Infof("Successfully updated lease")
}

func (c *NodeLeaseController) createLeaseIfNotExists(ctx context.Context, nodes []*corev1.Node) error {
	for _, node := range nodes {
		// namespaceName := types.NamespacedName{
		// 	Namespace: corev1.NamespaceNodeLease,
		// 	Name:      node.Name,
		// }
		_, err := c.rootClient.CoordinationV1().Leases(corev1.NamespaceNodeLease).Get(ctx, node.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				leaseToCreate := c.newLease(node)
				_, err := c.rootClient.CoordinationV1().Leases(leaseToCreate.Namespace).Create(ctx, leaseToCreate, metav1.CreateOptions{})
				if err != nil {
					klog.Errorf("create lease %s failed", node.Name)
					return err
				}
			} else {
				klog.Errorf("get lease %s failed, err: %s", node.Name, err)
				return err
			}
		}
	}
	return nil
}

func (c *NodeLeaseController) updateLeaseWithRetry(ctx context.Context, nodes []*corev1.Node) error {
	for _, node := range nodes {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			lease := &coordinationv1.Lease{}
			namespaceName := types.NamespacedName{
				Namespace: corev1.NamespaceNodeLease,
				Name:      node.Name,
			}
			if tmp, err := c.rootClient.CoordinationV1().Leases(namespaceName.Namespace).Get(ctx, namespaceName.Name, metav1.GetOptions{}); err != nil {
				klog.Warningf("get lease %s failed with err %v", node.Name, err)
				return err
			} else {
				lease = tmp
			}

			lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}
			lease.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
					Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
					Name:       node.Name,
					UID:        node.UID,
				},
			}
			_, err := c.rootClient.CoordinationV1().Leases(namespaceName.Namespace).Update(ctx, lease, metav1.UpdateOptions{})
			if err != nil {
				klog.Warningf("update lease %s failed with err %v", node.Name, err)
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *NodeLeaseController) newLease(node *corev1.Node) *coordinationv1.Lease {
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: corev1.NamespaceNodeLease,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
					Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
					Name:       node.Name,
					UID:        node.UID,
				},
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       pointer.String(node.Name),
			LeaseDurationSeconds: pointer.Int32(DefaultLeaseDuration),
			RenewTime:            &metav1.MicroTime{Time: time.Now()},
		},
	}
	return lease
}

func getRenewInterval() time.Duration {
	interval := DefaultLeaseDuration * DefaultRenewIntervalFraction
	intervalDuration := time.Second * time.Duration(int(interval))
	return intervalDuration
}
