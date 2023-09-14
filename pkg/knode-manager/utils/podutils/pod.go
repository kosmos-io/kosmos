package podutils

import (
	"strings"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

func IsEqual(pod1, pod2 *corev1.Pod) bool {
	return cmp.Equal(pod1.Spec.Containers, pod2.Spec.Containers) &&
		cmp.Equal(pod1.Spec.InitContainers, pod2.Spec.InitContainers) &&
		cmp.Equal(pod1.Spec.ActiveDeadlineSeconds, pod2.Spec.ActiveDeadlineSeconds) &&
		cmp.Equal(pod1.Spec.Tolerations, pod2.Spec.Tolerations) &&
		cmp.Equal(pod1.ObjectMeta.Labels, pod2.Labels) &&
		cmp.Equal(pod1.ObjectMeta.Annotations, pod2.Annotations)
}

func IsChange(pod1, pod2 *corev1.Pod) bool {
	return !(cmp.Equal(pod1.Status, pod2.Status) &&
		cmp.Equal(pod1.Annotations, pod2.Annotations) &&
		cmp.Equal(pod1.Labels, pod2.Labels) &&
		cmp.Equal(pod1.Finalizers, pod2.Finalizers))
}

func DeleteGraceTimeEqual(old, new *int64) bool {
	if old == nil && new == nil {
		return true
	}
	if old != nil && new != nil {
		return *old == *new
	}
	return false
}

func ShouldEnqueue(oldPod, newPod *corev1.Pod) bool {
	if !IsEqual(oldPod, newPod) {
		return true
	}
	if !DeleteGraceTimeEqual(oldPod.DeletionGracePeriodSeconds, newPod.DeletionGracePeriodSeconds) {
		return true
	}
	if !oldPod.DeletionTimestamp.Equal(newPod.DeletionTimestamp) {
		return true
	}
	return false
}

func ShouldSkipStatusUpdate(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded ||
		pod.Status.Phase == corev1.PodFailed
}

func IsRunning(podStatus *corev1.PodStatus) bool {
	statuses := podStatus.ContainerStatuses
	for _, status := range statuses {
		if status.State.Terminated == nil && status.State.Waiting == nil {
			return true
		}
	}
	return false
}

func LoggableName(pod *corev1.Pod) string {
	k, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		return "(unknown)"
	}
	return k
}

func EffectivelyEqual(p1, p2 *corev1.Pod) bool {
	filterForResourceVersion := func(p cmp.Path) bool {
		if p.String() == "ObjectMeta.ResourceVersion" {
			return true
		}
		if p.String() == "Status" {
			return true
		}
		return false
	}

	return cmp.Equal(p1, p2, cmp.FilterPath(filterForResourceVersion, cmp.Ignore()))
}

func GetUIDAndMetaNamespaceKey(key string) (string, string) {
	idx := strings.LastIndex(key, "/")
	uid := key[idx+1:]
	metaKey := key[:idx]
	return uid, metaKey
}
