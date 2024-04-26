package util

import v1 "k8s.io/api/core/v1"

func IsNodeReady(conditions []v1.NodeCondition) bool {
	for _, condition := range conditions {
		if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}
