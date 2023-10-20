package daemonset

import (
	"strconv"

	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	intstrutil "k8s.io/apimachinery/pkg/util/intstr"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

// GetTemplateGeneration gets the template generation associated with a v1.DaemonSet by extracting it from the
// deprecated annotation. If no annotation is found nil is returned. If the annotation is found and fails to parse
// nil is returned with an error. If the generation can be parsed from the annotation, a pointer to the parsed int64
// value is returned.
func GetTemplateGeneration(ds *kosmosv1alpha1.ShadowDaemonSet) (*int64, error) {
	annotation, found := ds.Annotations[apps.DeprecatedTemplateGeneration]
	if !found {
		return nil, nil
	}
	generation, err := strconv.ParseInt(annotation, 10, 64)
	if err != nil {
		return nil, err
	}
	return &generation, nil
}

// UnavailableCount returns 0 if unavailability is not requested, the expected
// unavailability number to allow out of numberToSchedule if requested, or an error if
// the unavailability percentage requested is invalid.
func UnavailableCount(ds *kosmosv1alpha1.ShadowDaemonSet, numberToSchedule int) (int, error) {
	if ds.DaemonSetSpec.UpdateStrategy.Type != apps.RollingUpdateDaemonSetStrategyType {
		return 0, nil
	}
	r := ds.DaemonSetSpec.UpdateStrategy.RollingUpdate
	if r == nil {
		return 0, nil
	}
	return intstrutil.GetScaledValueFromIntOrPercent(r.MaxUnavailable, numberToSchedule, true)
}

// SurgeCount returns 0 if surge is not requested, the expected surge number to allow
// out of numberToSchedule if surge is configured, or an error if the surge percentage
// requested is invalid.
func SurgeCount(ds *kosmosv1alpha1.ShadowDaemonSet, numberToSchedule int) (int, error) {
	if ds.DaemonSetSpec.UpdateStrategy.Type != apps.RollingUpdateDaemonSetStrategyType {
		return 0, nil
	}

	r := ds.DaemonSetSpec.UpdateStrategy.RollingUpdate
	if r == nil {
		return 0, nil
	}
	// If surge is not requested, we should default to 0.
	if r.MaxSurge == nil {
		return 0, nil
	}
	return intstrutil.GetScaledValueFromIntOrPercent(r.MaxSurge, numberToSchedule, true)
}

// AllowsSurge returns true if the daemonset allows more than a single pod on any node.
func AllowsSurge(ds *kosmosv1alpha1.ShadowDaemonSet) bool {
	maxSurge, err := SurgeCount(ds, 1)
	return err == nil && maxSurge > 0
}

// NewControllerRef creates an OwnerReference pointing to the given owner.
func NewControllerRef(owner metav1.Object, gvk schema.GroupVersionKind) *metav1.OwnerReference {
	blockOwnerDeletion := true
	isController := true
	return &metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,
	}
}

func isOwnedBy(references []metav1.OwnerReference, ds metav1.Object) bool {
	for i := range references {
		ref := references[i]
		if ref.UID == ds.GetUID() {
			return true
		}
	}
	return false
}
