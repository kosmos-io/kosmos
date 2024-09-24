// nolint:dupl
package convertpolicy

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

func TestGetMatchPodConvertPolicy(t *testing.T) {
	t.Run("Test with empty policies", func(t *testing.T) {
		policies := kosmosv1alpha1.PodConvertPolicyList{}
		podLabels := map[string]string{
			"app": "test",
		}
		nodeLabels := map[string]string{
			"node": "test",
		}

		matched, err := GetMatchPodConvertPolicy(policies, podLabels, nodeLabels)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(matched) != 0 {
			t.Errorf("Expected no matched policies, got %v", matched)
		}
	})

	t.Run("Test with policies that do not match", func(t *testing.T) {
		policies := kosmosv1alpha1.PodConvertPolicyList{
			Items: []kosmosv1alpha1.PodConvertPolicy{
				{
					Spec: kosmosv1alpha1.PodConvertPolicySpec{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "not-test",
							},
						},
					},
				},
			},
		}

		podLabels := map[string]string{
			"app": "test",
		}
		nodeLabels := map[string]string{
			"node": "test",
		}
		matched, err := GetMatchPodConvertPolicy(policies, podLabels, nodeLabels)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(matched) != 0 {
			t.Errorf("Expected no matched policies, got %v", matched)
		}
	})

	t.Run("Test with policies that match", func(t *testing.T) {
		policies := kosmosv1alpha1.PodConvertPolicyList{
			Items: []kosmosv1alpha1.PodConvertPolicy{
				{
					Spec: kosmosv1alpha1.PodConvertPolicySpec{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test",
							},
						},
					},
				},
			},
		}

		podLabels := map[string]string{
			"app": "test",
		}
		nodeLabels := map[string]string{
			"node": "test",
		}
		matched, err := GetMatchPodConvertPolicy(policies, podLabels, nodeLabels)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(matched) != 1 {
			t.Errorf("Expected 1 matched policy, got %v", len(matched))
		}
	})
}

func TestGetMatchClusterPodConvertPolicy(t *testing.T) {
	t.Run("Test with empty policies", func(t *testing.T) {
		policies := kosmosv1alpha1.ClusterPodConvertPolicyList{}
		podLabels := map[string]string{
			"app": "test",
		}
		nodeLabels := map[string]string{
			"node": "test",
		}

		matched, err := GetMatchClusterPodConvertPolicy(policies, podLabels, nodeLabels)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(matched) != 0 {
			t.Errorf("Expected no matched policies, got %v", matched)
		}
	})

	t.Run("Test with policies that do not match", func(t *testing.T) {
		policies := kosmosv1alpha1.ClusterPodConvertPolicyList{
			Items: []kosmosv1alpha1.ClusterPodConvertPolicy{
				{
					Spec: kosmosv1alpha1.ClusterPodConvertPolicySpec{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "not-test"},
						},
					},
				},
			},
		}

		podLabels := map[string]string{
			"app": "test",
		}
		nodeLabels := map[string]string{
			"node": "test",
		}
		matched, err := GetMatchClusterPodConvertPolicy(policies, podLabels, nodeLabels)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(matched) != 0 {
			t.Errorf("Expected no matched policies, got %v", matched)
		}
	})

	t.Run("Test with policies that match", func(t *testing.T) {
		policies := kosmosv1alpha1.ClusterPodConvertPolicyList{
			Items: []kosmosv1alpha1.ClusterPodConvertPolicy{
				{
					Spec: kosmosv1alpha1.ClusterPodConvertPolicySpec{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
					},
				},
			},
		}

		podLabels := map[string]string{
			"app": "test",
		}
		nodeLabels := map[string]string{
			"node": "test",
		}
		matched, err := GetMatchClusterPodConvertPolicy(policies, podLabels, nodeLabels)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(matched) != 1 {
			t.Errorf("more than one matched policies, got %v", matched)
		}
	})
}
