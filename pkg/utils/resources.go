package utils

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1resource "k8s.io/kubernetes/pkg/api/v1/resource"
)

const (
	podResourceName corev1.ResourceName = "pods"
)

func CalculateClusterResources(nodes *corev1.NodeList, pods *corev1.PodList) corev1.ResourceList {
	base := GetNodesTotalResources(nodes)
	reqs, _ := GetPodsTotalRequestsAndLimits(pods)
	podNums := GetUsedPodNums(pods, nodes)
	SubResourceList(base, reqs)
	SubResourceList(base, podNums)
	return base
}

func GetNodesTotalResources(nodes *corev1.NodeList) (total corev1.ResourceList) {
	total = corev1.ResourceList{}
	for i, n := range nodes.Items {
		if n.Spec.Unschedulable || !NodeReady(&nodes.Items[i]) {
			continue
		}
		for key, val := range n.Status.Allocatable {
			if value, ok := total[key]; !ok {
				total[key] = val.DeepCopy()
			} else {
				value.Add(val)
				total[key] = value
			}
		}
	}
	return
}

func SubResourceList(base, list corev1.ResourceList) {
	for name, quantity := range list {
		value, ok := base[name]
		if ok {
			q := value.DeepCopy()
			q.Sub(quantity)
			base[name] = q
		}
	}
}

// GetPodsTotalRequestsAndLimits
// lifted from https://github.com/kubernetes/kubernetes/blob/v1.21.8/staging/src/k8s.io/kubectl/pkg/describe/describe.go#L4051
func GetPodsTotalRequestsAndLimits(podList *corev1.PodList) (reqs corev1.ResourceList, limits corev1.ResourceList) {
	reqs, limits = corev1.ResourceList{}, corev1.ResourceList{}
	if podList.Items != nil {
		for _, p := range podList.Items {
			pod := p
			if IsVirtualPod(&pod) {
				continue
			}
			podReqs, podLimits := v1resource.PodRequestsAndLimits(&pod)
			for podReqName, podReqValue := range podReqs {
				if value, ok := reqs[podReqName]; !ok {
					reqs[podReqName] = podReqValue.DeepCopy()
				} else {
					value.Add(podReqValue)
					reqs[podReqName] = value
				}
			}
			for podLimitName, podLimitValue := range podLimits {
				if value, ok := limits[podLimitName]; !ok {
					limits[podLimitName] = podLimitValue.DeepCopy()
				} else {
					value.Add(podLimitValue)
					limits[podLimitName] = value
				}
			}
		}
	}
	return
}

func GetUsedPodNums(podList *corev1.PodList, nodes *corev1.NodeList) (res corev1.ResourceList) {
	podQuantity := resource.Quantity{}
	res = corev1.ResourceList{}
	nodeMap := map[string]corev1.Node{}
	for _, item := range nodes.Items {
		nodeMap[item.Name] = item
	}
	for _, p := range podList.Items {
		pod := p
		if IsVirtualPod(&pod) {
			continue
		}
		node, exists := nodeMap[pod.Spec.NodeName]
		if !exists || node.Spec.Unschedulable || !NodeReady(&node) {
			continue
		}
		q := resource.MustParse("1")
		podQuantity.Add(q)
	}
	res[podResourceName] = podQuantity
	return
}
