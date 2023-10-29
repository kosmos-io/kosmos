package utils

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BuildNodeTemplate(name string) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				KosmosNodeLabel:     KosmosNodeValue,
				NodeRoleLabel:       NodeRoleValue,
				NodeHostnameValue:   name,
				NodeArchLabelStable: DefaultK8sArch,
				NodeOSLabelStable:   DefaultK8sOS,
				NodeOSLabelBeta:     DefaultK8sOS,
			},
		},
		Spec: corev1.NodeSpec{},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				OperatingSystem: DefaultK8sOS,
				Architecture:    DefaultK8sArch,
			},
		},
	}

	// todo: custom taints from cluster cr
	node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
		Key:    KosmosNodeTaintKey,
		Value:  KosmosNodeTaintValue,
		Effect: KosmosNodeTaintEffect,
	})

	node.Status.Conditions = NodeConditions()

	return node
}

func NodeConditions() []corev1.NodeCondition {
	return []corev1.NodeCondition{
		{
			Type:               "Ready",
			Status:             corev1.ConditionTrue,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletReady",
			Message:            "kubelet is posting ready status",
		},
		{
			Type:               "NetworkUnavailable",
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "RouteCreated",
			Message:            "RouteController created a route",
		},
		{
			Type:               "MemoryPressure",
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientMemory",
			Message:            "kubelet has sufficient memory available",
		},
		{
			Type:               "DiskPressure",
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasNoDiskPressure",
			Message:            "kubelet has no disk pressure",
		},
		{
			Type:               "PIDPressure",
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientPID",
			Message:            "kubelet has sufficient PID available",
		},
	}
}

func NodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type != corev1.NodeReady {
			continue
		}
		if condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

//func UpdateLastHeartbeatTime(n *corev1.Node) {
//	now := metav1.NewTime(time.Now())
//	for i := range n.Status.Conditions {
//		n.Status.Conditions[i].LastHeartbeatTime = now
//	}
//}
