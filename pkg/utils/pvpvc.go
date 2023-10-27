package utils

import (
	"reflect"

	v1 "k8s.io/api/core/v1"
)

func IsPVEqual(pv *v1.PersistentVolume, clone *v1.PersistentVolume) bool {
	if reflect.DeepEqual(pv.Annotations, clone.Annotations) &&
		reflect.DeepEqual(pv.Spec, clone.Spec) &&
		reflect.DeepEqual(pv.Status, clone.Status) {
		return true
	}
	return false
}
