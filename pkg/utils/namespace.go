package utils

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func FitNs(ns *corev1.Namespace) *corev1.Namespace {
	nsCopy := ns.DeepCopy()
	FitObjectMeta(&nsCopy.ObjectMeta)
	nsCopy.Labels[KosmosNameSpaceLabel] = "true"
	return nsCopy
}

func FitObjectMeta(meta *metav1.ObjectMeta) {
	meta.UID = ""
	meta.ResourceVersion = ""
	meta.OwnerReferences = nil
}
