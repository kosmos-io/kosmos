package controllers

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Enqueue(t *testing.T) {
	const name = "public"

	worker := NewWorker(nil, "namespace")

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}

	worker.Enqueue(ns)

	first, _ := worker.GetFirst()

	_, metaName, _ := worker.SplitKey(first)

	if name != metaName {
		t.Errorf("Added NS: %v, want: %v, but resout: %v", first, name, metaName)
	}
}
