package convertpolicy

import (
	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetMatchPodConvertPolicy returns the PodConvertPolicies matching label selector
func GetMatchPodConvertPolicy(policies kosmosv1alpha1.PodConvertPolicyList, podLabels map[string]string, nodeLabels map[string]string) ([]kosmosv1alpha1.PodConvertPolicy, error) {
	matched := make([]kosmosv1alpha1.PodConvertPolicy, 0)

	var podSelector, nodeSelector labels.Selector
	var err error
	for _, policy := range policies.Items {
		podSelector, err = metav1.LabelSelectorAsSelector(&policy.Spec.LabelSelector)
		if err != nil {
			return nil, err
		}
		if !podSelector.Matches(labels.Set(podLabels)) {
			continue
		}

		if policy.Spec.LeafNodeSelector == nil {
			// matches all leafNode.
			nodeSelector = labels.Everything()
		} else {
			if nodeSelector, err = metav1.LabelSelectorAsSelector(policy.Spec.LeafNodeSelector); err != nil {
				return nil, err
			}
		}
		if !nodeSelector.Matches(labels.Set(nodeLabels)) {
			continue
		}

		matched = append(matched, policy)
	}
	return matched, nil
}
