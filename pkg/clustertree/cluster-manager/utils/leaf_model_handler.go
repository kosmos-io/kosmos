package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

// LeafModelHandler is the interface to handle the leafModel logic
type LeafModelHandler interface {
	// GetLeafModelType returns the leafModelType for a Cluster
	GetLeafModelType() LeafModelType

	// GetGlobalLeafManagerClusterName returns the clusterName for a Cluster's GlobalLeafManager
	GetGlobalLeafManagerClusterName(cluster *kosmosv1alpha1.Cluster) string

	// GetLeafNodes returns nodes in leaf cluster by the rootNode
	GetLeafNodes(ctx context.Context, rootNode *corev1.Node) (*corev1.NodeList, error)

	// GetLeafPods returns pods in leaf cluster by the rootNode
	GetLeafPods(ctx context.Context, rootNode *corev1.Node) (*corev1.PodList, error)

	// UpdateNodeStatus updates the node's status in root cluster
	UpdateNodeStatus(ctx context.Context, node []*corev1.Node) error

	// CreateNodeInRoot creates the node in root cluster
	CreateNodeInRoot(ctx context.Context, cluster *kosmosv1alpha1.Cluster, listenPort int32, gitVersion string) ([]*corev1.Node, error)
}

// LeafModelType represents the type of leaf model
type LeafModelType string

const (
	AggregationModel LeafModelType = "aggregation"
	DispersionModel  LeafModelType = "dispersion"
)

// AggregationModelHandler handles the aggregation leaf model
type AggregationModelHandler struct {
	Cluster       *kosmosv1alpha1.Cluster
	LeafClient    client.Client
	RootClient    client.Client
	RootClientset kubernetes.Interface
}

// CreateNodeInRoot creates the node in root cluster
func (h AggregationModelHandler) CreateNodeInRoot(ctx context.Context, cluster *kosmosv1alpha1.Cluster, listenPort int32, gitVersion string) ([]*corev1.Node, error) {
	nodes := make([]*corev1.Node, 0)
	nodeName := fmt.Sprintf("%s%s", utils.KosmosNodePrefix, cluster.Name)
	node, err := h.RootClientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			node = utils.BuildNodeTemplate(nodeName)
			node.Status.NodeInfo.KubeletVersion = gitVersion
			node.Status.DaemonEndpoints = corev1.NodeDaemonEndpoints{
				KubeletEndpoint: corev1.DaemonEndpoint{
					Port: listenPort,
				},
			}

			node.Status.Addresses = GetAddress()

			node, err = h.RootClientset.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					return nil, err
				} else {
					nodes = append(nodes, node)
				}
			}
		} else {
			return nil, err
		}
	} else {
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// UpdateNodeStatus updates the node's status in root cluster
func (h AggregationModelHandler) UpdateNodeStatus(ctx context.Context, n []*corev1.Node) error {
	var name string
	if len(n) > 0 {
		name = n[0].Name
	}

	node := &corev1.Node{}
	namespacedName := types.NamespacedName{
		Name: name,
	}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := h.RootClient.Get(ctx, namespacedName, node)
		if err != nil {
			// TODO: If a node is accidentally deleted, recreate it
			return fmt.Errorf("cannot get node while update node status %s, err: %v", name, err)
		}

		clone := node.DeepCopy()
		clone.Status.Conditions = utils.NodeConditions()

		patch, err := utils.CreateMergePatch(node, clone)
		if err != nil {
			return fmt.Errorf("cannot get node while update node status %s, err: %v", node.Name, err)
		}

		if node, err = h.RootClientset.CoreV1().Nodes().PatchStatus(ctx, node.Name, patch); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// GetLeafPods returns pods in leaf cluster by the rootNode
func (h AggregationModelHandler) GetLeafPods(ctx context.Context, rootNode *corev1.Node) (*corev1.PodList, error) {
	pods := &corev1.PodList{}
	err := h.LeafClient.List(ctx, pods)
	if err != nil {
		return nil, err
	}
	return pods, nil
}

// GetLeafNodes returns nodes in leaf cluster by the rootNode
func (h AggregationModelHandler) GetLeafNodes(ctx context.Context, _ *corev1.Node) (*corev1.NodeList, error) {
	nodesInLeaf := &corev1.NodeList{}
	err := h.LeafClient.List(ctx, nodesInLeaf)
	if err != nil {
		return nil, err
	}
	return nodesInLeaf, nil
}

// GetGlobalLeafManagerClusterName returns the clusterName for a Cluster's GlobalLeafManager
func (h AggregationModelHandler) GetGlobalLeafManagerClusterName(cluster *kosmosv1alpha1.Cluster) string {
	clusterName := fmt.Sprintf("%s%s", utils.KosmosNodePrefix, cluster.Name)
	return clusterName
}

// GetLeafModelType returns the leafModelType for a Cluster
func (h AggregationModelHandler) GetLeafModelType() LeafModelType {
	return AggregationModel
}

// DispersionModelHandler handles the dispersion leaf model
type DispersionModelHandler struct {
	Cluster       *kosmosv1alpha1.Cluster
	LeafClient    client.Client
	RootClient    client.Client
	RootClientset kubernetes.Interface
	LeafClientset kubernetes.Interface
}

// CreateNodeInRoot creates the node in root cluster
func (h DispersionModelHandler) CreateNodeInRoot(ctx context.Context, cluster *kosmosv1alpha1.Cluster, listenPort int32, gitVersion string) ([]*corev1.Node, error) {
	nodes := make([]*corev1.Node, 0)
	for _, leafModel := range cluster.Spec.ClusterTreeOptions.LeafModels {
		// todo only support nodeName now
		if leafModel.NodeSelector.NodeName != "" {
			nodeName := leafModel.NodeSelector.NodeName

			node, err := h.RootClientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					node = utils.BuildNodeTemplate(nodeName)

					nodeAnnotations := node.GetAnnotations()
					if nodeAnnotations == nil {
						nodeAnnotations = make(map[string]string, 1)
					}
					nodeAnnotations[utils.KosmosNodeOwnedByClusterAnnotations] = cluster.Name
					node.SetAnnotations(nodeAnnotations)

					node.Status.NodeInfo.KubeletVersion = gitVersion
					node.Status.DaemonEndpoints = corev1.NodeDaemonEndpoints{
						KubeletEndpoint: corev1.DaemonEndpoint{
							Port: listenPort,
						},
					}

					node.Status.Addresses = GetAddress()

					node, err = h.RootClientset.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
					if err != nil {
						if !errors.IsAlreadyExists(err) {
							return nil, err
						} else {
							nodes = append(nodes, node)
						}
					}
				} else {
					return nil, err
				}
			} else {
				nodes = append(nodes, node)
			}
		}
	}
	return nodes, nil
}

// UpdateNodeStatus updates the node's status in root cluster
func (h DispersionModelHandler) UpdateNodeStatus(ctx context.Context, n []*corev1.Node) error {
	for _, node := range n {
		nodeCopy := node.DeepCopy()
		namespacedName := types.NamespacedName{
			Name: nodeCopy.Name,
		}
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			nodeInLeaf := &corev1.Node{}
			err := h.LeafClient.Get(ctx, namespacedName, nodeInLeaf)
			if err != nil {
				// TODO: If a node is accidentally deleted, recreate it
				return fmt.Errorf("cannot get node in leaf cluster while update node status %s, err: %v", nodeCopy.Name, err)
			}

			nodeRoot := &corev1.Node{}
			err = h.RootClient.Get(ctx, namespacedName, nodeRoot)
			if err != nil {
				// TODO: If a node is accidentally deleted, recreate it
				return fmt.Errorf("cannot get node in root cluster while update node status %s, err: %v", nodeCopy.Name, err)
			}

			nodeRoot.Status = nodeInLeaf.Status
			nodeRoot.Status.Addresses = GetAddress()

			if node, err = h.RootClientset.CoreV1().Nodes().UpdateStatus(ctx, nodeRoot, metav1.UpdateOptions{}); err != nil {
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

func (h DispersionModelHandler) GetLeafPods(ctx context.Context, rootNode *corev1.Node) (*corev1.PodList, error) {
	pods, err := h.LeafClientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("spec.nodeName=%s", rootNode.Name)})
	if err != nil {
		return nil, err
	}
	return pods, nil
}

func (h DispersionModelHandler) GetLeafNodes(ctx context.Context, rootNode *corev1.Node) (*corev1.NodeList, error) {
	nodesInLeaf, err := h.LeafClientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", rootNode.Name)})
	if err != nil {
		return nil, err
	}
	return nodesInLeaf, nil
}

func (h DispersionModelHandler) GetGlobalLeafManagerClusterName(cluster *kosmosv1alpha1.Cluster) string {
	return cluster.Name
}

func (h DispersionModelHandler) GetLeafModelType() LeafModelType {
	return DispersionModel
}

// NewLeafModelHandler create a LeafModelHandler for Cluster
func NewLeafModelHandler(cluster *kosmosv1alpha1.Cluster, root, leafClient client.Client, rootClientset, leafClientset kubernetes.Interface) LeafModelHandler {
	// todo support nodeSelector mode
	if cluster.Spec.ClusterTreeOptions.LeafModels != nil {
		return &DispersionModelHandler{
			Cluster:       cluster,
			LeafClient:    leafClient,
			RootClient:    root,
			RootClientset: rootClientset,
			LeafClientset: leafClientset,
		}
	} else {
		return &AggregationModelHandler{
			Cluster:       cluster,
			LeafClient:    leafClient,
			RootClient:    root,
			RootClientset: rootClientset,
		}
	}
}
