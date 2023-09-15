package utils

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func SetObjectGlobal(obj *metav1.ObjectMeta) {
	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	obj.Annotations[KosmosGlobalLabel] = "true"
}
