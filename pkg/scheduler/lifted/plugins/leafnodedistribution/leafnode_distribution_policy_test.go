package leafnodedistribution

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

func TestMatchDistributionPolicy(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
	}

	dp := &kosmosv1alpha1.DistributionPolicy{
		DistributionSpec: kosmosv1alpha1.DistributionSpec{
			ResourceSelectors: []kosmosv1alpha1.ResourceSelector{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
					PolicyName: "test-policy",
				},
			},
		},
	}

	policyName, priority, _ := matchDistributionPolicy(pod, dp)
	if policyName == "test-policy" {
		t.Errorf("Expected policy name to be 'test-policy', got '%s'", policyName)
	}
	if priority == LabelSelectorInNsScope {
		t.Errorf("Expected priority to be LabelSelectorInNsScope, got %d", priority)
	}
}
