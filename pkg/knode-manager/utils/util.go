package utils

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func SetObjectGlobal(obj *metav1.ObjectMeta) {
	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	obj.Annotations[KosmosGlobalLabel] = "true"
}

func IsObjectGlobal(obj *metav1.ObjectMeta) bool {
	if obj.Annotations == nil {
		return false
	}

	if obj.Annotations[KosmosGlobalLabel] == "true" {
		return true
	}

	return false
}

func CheckGlobalLabelEqual(obj, clone *metav1.ObjectMeta) bool {
	oldGlobal := IsObjectGlobal(obj)
	if !oldGlobal {
		return false
	}
	newGlobal := IsObjectGlobal(clone)
	return newGlobal
}
