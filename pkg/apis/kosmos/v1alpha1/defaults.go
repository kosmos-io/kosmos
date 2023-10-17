package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func SetDefaults_ShadowDaemonSet(obj *ShadowDaemonSet) {
	updateStrategy := &obj.DaemonSetSpec.UpdateStrategy
	if updateStrategy.Type == "" {
		updateStrategy.Type = appsv1.RollingUpdateDaemonSetStrategyType
	}
	if updateStrategy.Type == appsv1.RollingUpdateDaemonSetStrategyType {
		if updateStrategy.RollingUpdate == nil {
			rollingUpdate := appsv1.RollingUpdateDaemonSet{}
			updateStrategy.RollingUpdate = &rollingUpdate
		}
		if updateStrategy.RollingUpdate.MaxUnavailable == nil {
			// Set default MaxUnavailable as 1 by default.
			maxUnavailable := intstr.FromInt(1)
			updateStrategy.RollingUpdate.MaxUnavailable = &maxUnavailable
		}
		if updateStrategy.RollingUpdate.MaxSurge == nil {
			// Set default MaxSurge as 0 by default.
			maxSurge := intstr.FromInt(0)
			updateStrategy.RollingUpdate.MaxSurge = &maxSurge
		}
	}
	if obj.DaemonSetSpec.RevisionHistoryLimit == nil {
		obj.DaemonSetSpec.RevisionHistoryLimit = new(int32)
		*obj.DaemonSetSpec.RevisionHistoryLimit = 10
	}
}
