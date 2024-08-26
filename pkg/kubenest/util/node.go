package util

import (
	"context"
	"fmt"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
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
