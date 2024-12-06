package util

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	drain "k8s.io/kubectl/pkg/drain"
)

func IsNodeReady(conditions []v1.NodeCondition) bool {
	for _, condition := range conditions {
		if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// DrainNode cordons and drains a node.
func DrainNode(ctx context.Context, nodeName string, client kubernetes.Interface, node *v1.Node, drainWaitSeconds int, isHostCluster bool) error {
	if client == nil {
		return fmt.Errorf("K8sClient not set")
	}
	if node == nil {
		return fmt.Errorf("node not set")
	}
	if nodeName == "" {
		return fmt.Errorf("node name not set")
	}
	helper := &drain.Helper{
		Ctx:                 ctx,
		Client:              client,
		Force:               true,
		GracePeriodSeconds:  -1,
		IgnoreAllDaemonSets: true,
		Out:                 os.Stdout,
		ErrOut:              os.Stdout,
		DisableEviction:     !isHostCluster,
		// We want to proceed even when pods are using emptyDir volumes
		DeleteEmptyDirData: true,
		Timeout:            time.Duration(drainWaitSeconds) * time.Second,
	}
	if err := drain.RunCordonOrUncordon(helper, node, true); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error cordoning node: %v", err)
	}
	if err := drain.RunNodeDrain(helper, nodeName); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error draining node: %v", err)
	}
	return nil
}

func GetAPIServerNodes(rootClientSet kubernetes.Interface, namespace string) (*v1.NodeList, error) {
	klog.V(4).Info("begin to get API server nodes")

	apiServerPods, err := rootClientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "virtualCluster-app=apiserver",
	})
	if err != nil {
		klog.Errorf("failed to list kube-apiserver pod: %v", err)
		return nil, errors.Wrap(err, "failed to list kube-apiserver pods")
	}

	var nodeNames []string
	for _, pod := range apiServerPods.Items {
		klog.V(4).Infof("API server pod %s is on node: %s", pod.Name, pod.Spec.NodeName)
		nodeNames = append(nodeNames, pod.Spec.NodeName)
	}

	if len(nodeNames) == 0 {
		klog.Errorf("no API server pods found in the namespace")
		return nil, fmt.Errorf("no API server pods found")
	}

	var nodesList []v1.Node
	for _, nodeName := range nodeNames {
		node, err := rootClientSet.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("failed to get node %s: %v", nodeName, err)
			return nil, fmt.Errorf("failed to get node %s: %v", nodeName, err)
		}
		klog.V(4).Infof("Found node: %s", node.Name)
		nodesList = append(nodesList, *node)
	}

	nodes := &v1.NodeList{
		Items: nodesList,
	}

	klog.V(4).Infof("got %d API server nodes", len(nodes.Items))

	if len(nodes.Items) == 0 {
		klog.Errorf("no nodes found for the API server pods")
		return nil, fmt.Errorf("no nodes found for the API server pods")
	}

	return nodes, nil
}
