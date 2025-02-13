package helpers

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

func TestConvertPolicy(t *testing.T) {
	policies := []v1alpha1.AllocationPolicy{
		{
			Name:     "policy1",
			Replicas: 1,
		},
		{
			Name:     "policy2",
			Replicas: 2,
		},
	}

	expected := map[string]int32{
		"policy1": 1,
		"policy2": 2,
	}

	result := ConvertPolicy(policies)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, but got %v", expected, result)
	}
}

func TestHasWorkloadPolicyLabel(t *testing.T) {
	podWithLabel := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				WorkloadPolicyLabelKey: "true",
			},
		},
	}
	podWithoutLabel := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{},
		},
	}

	if !HasWorkloadPolicyLabel(podWithLabel) {
		t.Errorf("expected true, but got false")
	}

	if HasWorkloadPolicyLabel(podWithoutLabel) {
		t.Errorf("expected false, but got true")
	}
}

func TestCountPodsMatchSelector(t *testing.T) {
	podInfos := []*framework.PodInfo{
		{
			Pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "ns1",
					Labels: map[string]string{
						"app": "test",
					},
				},
			},
		},
		{
			Pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod2",
					Namespace: "ns1",
					Labels: map[string]string{
						"app": "test",
					},
				},
			},
		},
		{
			Pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod3",
					Namespace: "ns2",
					Labels: map[string]string{
						"app": "test",
					},
				},
			},
		},
		{
			Pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod4",
					Namespace: "ns1",
					Labels: map[string]string{
						"app": "other",
					},
				},
			},
		},
	}

	selector := labels.SelectorFromSet(labels.Set{"app": "test"})
	count := CountPodsMatchSelector(podInfos, selector, "ns1")
	if count != 2 {
		t.Errorf("expected 2, but got %d", count)
	}
}

func TestCheckTopologyValueReached(t *testing.T) {
	zone1 := int32(1)
	zone2 := int32(3)

	allocationPolicy := map[string]int32{
		"zone1": 2,
		"zone2": 3,
		"zone4": 4,
	}
	tpPairToMatchNum := map[TopologyPair]*int32{
		{
			Key:   "region",
			Value: "zone1",
		}: &zone1,
		{
			Key:   "region",
			Value: "zone2",
		}: &zone2,
	}

	// Test case 1: count is less than desired
	reached, err := CheckTopologyValueReached("region", "zone1", allocationPolicy, tpPairToMatchNum)
	if err != nil {
		t.Errorf("Test case 1: expected no error, but got %v", err)
	}
	if reached {
		t.Errorf("Test case 1: expected false, but got true")
	}

	// Test case 2: count is equal to desired
	reached, err = CheckTopologyValueReached("region", "zone2", allocationPolicy, tpPairToMatchNum)
	if err != nil {
		t.Errorf("Test case 2: expected no error, but got %v", err)
	}
	if !reached {
		t.Errorf("Test case 2: expected true, but got false")
	}

	// Test case 3: tpValue not in allocationPolicy
	reached, err = CheckTopologyValueReached("region", "zone3", allocationPolicy, tpPairToMatchNum)
	if err == nil {
		t.Errorf("Test case 3: expected error, but got nil")
	}
	if !reached {
		t.Errorf("Test case 3: expected true, but got false")
	}

	// Test case 4: tpValue not in tpPairToMatchNum
	reached, err = CheckTopologyValueReached("region", "zone4", allocationPolicy, tpPairToMatchNum)
	if err != nil {
		t.Errorf("Test case 4: expected error, but got nil")
	}
	if reached {
		t.Errorf("Test case 4: expected false, but got true")
	}
}
