package leafnodeworkloadpolicy

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/helpers"
)

func TestUpdatePreFilterStateWithPod(t *testing.T) {
	zone1 := int32(1)
	zone2 := int32(3)

	// Create a PreFilterState object
	pfs := &PreFilterState{
		Constraint: helpers.WorkloadPolicyConstraint{
			TopologyKey: "region",
			Selector:    labels.Everything(),
		},
		TpPairToMatchNum: map[helpers.TopologyPair]*int32{
			{
				Key:   "region",
				Value: "zone1",
			}: &zone1,
			{
				Key:   "region",
				Value: "zone2",
			}: &zone2,
		},
	}

	// Create a node with a topology label
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Labels: map[string]string{
				"region": "zone1",
			},
		},
	}

	// Create a pod with a matching label
	updatedPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
	}

	// Create a preempt pod with the same namespace
	preemptPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod2",
			Namespace: "default",
		},
	}

	// Call the updatePreFilterStateWithPod method with a positive delta
	pfs.updatePreFilterStateWithPod(updatedPod, preemptPod, node, 1)
	if *pfs.TpPairToMatchNum[helpers.TopologyPair{Key: "region", Value: "zone1"}] != 2 {
		t.Errorf("expected 2, but got %d", *pfs.TpPairToMatchNum[helpers.TopologyPair{Key: "region", Value: "zone1"}])
	}

	// Call the updatePreFilterStateWithPod method with a negative delta
	pfs.updatePreFilterStateWithPod(updatedPod, preemptPod, node, -1)
	if *pfs.TpPairToMatchNum[helpers.TopologyPair{Key: "region", Value: "zone1"}] != 1 {
		t.Errorf("expected 1, but got %d", *pfs.TpPairToMatchNum[helpers.TopologyPair{Key: "region", Value: "zone1"}])
	}

	// Call the updatePreFilterStateWithPod method with a node that doesn't have the topology label
	node2 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
		},
	}
	pfs.updatePreFilterStateWithPod(updatedPod, preemptPod, node2, 1)
	if *pfs.TpPairToMatchNum[helpers.TopologyPair{Key: "region", Value: "zone1"}] != 1 {
		t.Errorf("expected 1, but got %d", *pfs.TpPairToMatchNum[helpers.TopologyPair{Key: "region", Value: "zone1"}])
	}
}
