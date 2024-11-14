package utils

import corev1 "k8s.io/api/core/v1"

func HasKosmosNodeLabel(node *corev1.Node) bool {
	if kosmosNodeLabel, ok := node.Labels[KosmosNodeLabel]; ok && kosmosNodeLabel == KosmosNodeValue {
		return true
	}

	return false
}
