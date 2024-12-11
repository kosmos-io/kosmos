package utils

import (
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

type clusterData struct {
	// readyCondition is the last observed ready condition of the cluster.
	readyCondition corev1.ConditionStatus
	// thresholdStartTime is the time that the ready condition changed.
	thresholdStartTime time.Time
}

func (c *clusterConditionStore) thresholdAdjustedReadyCondition(cluster *kosmosv1alpha1.Cluster, nodeInRoot *corev1.Node, observedReadyConditions []corev1.NodeCondition, clusterFailureThreshold time.Duration, clusterSuccessThreshold time.Duration) []corev1.NodeCondition {
	c.successThreshold = clusterSuccessThreshold
	c.failureThreshold = clusterFailureThreshold
	//Find the ready condition of the node todo: optimize the code format
	observedReadyCondition := FindStatusCondition(observedReadyConditions)
	if observedReadyCondition == nil {
		return observedReadyConditions
	}
	//Get the current status of the cluster (rootnode)
	curReadyConditions := nodeInRoot.Status.Conditions
	curReadyCondition := FindStatusCondition(curReadyConditions)
	//Get the saved data
	saved := c.get(cluster.Name)
	//Check whether it is a newly added cluster
	if saved == nil {
		// the cluster is just joined
		c.update(cluster.Name, &clusterData{
			readyCondition: observedReadyCondition.Status,
		})
		return observedReadyConditions
	}
	//Check if the state has changed
	now := time.Now()
	if saved.readyCondition != observedReadyCondition.Status {
		// ready condition status changed, record the threshold start time
		saved = &clusterData{
			readyCondition:     observedReadyCondition.Status,
			thresholdStartTime: now,
		}
		c.update(cluster.Name, saved)
	}
	//Setting time thresholds
	var threshold time.Duration
	if observedReadyCondition.Status == corev1.ConditionTrue {
		threshold = c.successThreshold
	} else {
		threshold = c.failureThreshold
	}
	if ((observedReadyCondition.Status == corev1.ConditionTrue && curReadyCondition.Status != corev1.ConditionTrue) ||
		(observedReadyCondition.Status != corev1.ConditionTrue && curReadyCondition.Status == corev1.ConditionTrue)) &&
		now.Before(saved.thresholdStartTime.Add(threshold)) {
		// retain old status until threshold exceeded to avoid network unstable problems.
		return curReadyConditions
	}
	return observedReadyConditions
}

// FindStatusCondition finds the conditionType in conditions.
func FindStatusCondition(conditions []corev1.NodeCondition) *corev1.NodeCondition {
	for i := range conditions {
		if conditions[i].Type == "Ready" {
			return &conditions[i]
		}
	}
	return nil
}

func (c *clusterConditionStore) get(cluster string) *clusterData {
	condition, ok := c.clusterDataMap.Load(cluster)
	if !ok {
		return nil
	}
	return condition.(*clusterData)
}

func (c *clusterConditionStore) update(cluster string, data *clusterData) {
	// ready condition status changed, record the threshold start time
	c.clusterDataMap.Store(cluster, data)
}

type clusterConditionStore struct {
	clusterDataMap   sync.Map
	successThreshold time.Duration
	// failureThreshold is the duration of failure for the cluster to be considered unhealthy.
	failureThreshold time.Duration
}
