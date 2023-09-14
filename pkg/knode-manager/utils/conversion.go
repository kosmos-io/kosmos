package utils

import (
	"encoding/json"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TrimPod(pod *corev1.Pod, ignoreLabels []string) *corev1.Pod {
	vols := []corev1.Volume{}
	for _, v := range pod.Spec.Volumes {
		if strings.HasPrefix(v.Name, "default-token") {
			continue
		}
		vols = append(vols, v)
	}

	podCopy := pod.DeepCopy()
	TrimObjectMeta(&podCopy.ObjectMeta)
	if podCopy.Labels == nil {
		podCopy.Labels = make(map[string]string)
	}
	if podCopy.Annotations == nil {
		podCopy.Annotations = make(map[string]string)
	}
	podCopy.Labels[KosmosPodLabel] = "true"
	cns := ConvertAnnotations(pod.Annotations)
	recoverSelectors(podCopy, cns)
	podCopy.Spec.Containers = trimContainers(pod.Spec.Containers)
	podCopy.Spec.InitContainers = trimContainers(pod.Spec.InitContainers)
	podCopy.Spec.Volumes = vols
	podCopy.Spec.NodeName = ""
	podCopy.Status = corev1.PodStatus{}

	podCopy.Spec.SchedulerName = ""

	tripped := TrimLabels(podCopy.ObjectMeta.Labels, ignoreLabels)
	if tripped != nil {
		trippedStr, err := json.Marshal(tripped)
		if err != nil {
			return podCopy
		}
		podCopy.Annotations[TrippedLabels] = string(trippedStr)
	}

	return podCopy
}

func trimContainers(containers []corev1.Container) []corev1.Container {
	var newContainers []corev1.Container

	for _, c := range containers {
		var volMounts []corev1.VolumeMount
		for _, v := range c.VolumeMounts {
			if strings.HasPrefix(v.Name, "default-token") {
				continue
			}
			volMounts = append(volMounts, v)
		}
		c.VolumeMounts = volMounts
		newContainers = append(newContainers, c)
	}

	return newContainers
}

func GetUpdatedPod(orig, update *corev1.Pod, ignoreLabels []string) {
	for i := range orig.Spec.InitContainers {
		orig.Spec.InitContainers[i].Image = update.Spec.InitContainers[i].Image
	}
	for i := range orig.Spec.Containers {
		orig.Spec.Containers[i].Image = update.Spec.Containers[i].Image
	}
	if update.Annotations == nil {
		update.Annotations = make(map[string]string)
	}
	if orig.Annotations[SelectorKey] != update.Annotations[SelectorKey] {
		if cns := ConvertAnnotations(update.Annotations); cns != nil {
			orig.Spec.Tolerations = cns.Tolerations
		}
	}
	orig.Labels = update.Labels
	orig.Annotations = update.Annotations
	orig.Spec.ActiveDeadlineSeconds = update.Spec.ActiveDeadlineSeconds
	if orig.Labels != nil {
		TrimLabels(orig.ObjectMeta.Labels, ignoreLabels)
	}
}

func TrimObjectMeta(meta *metav1.ObjectMeta) {
	meta.UID = ""
	meta.ResourceVersion = ""
	// meta.SelfLink = ""
	meta.OwnerReferences = nil
}

func RecoverLabels(labels map[string]string, annotations map[string]string) {
	trippedLabels := annotations[TrippedLabels]
	if trippedLabels == "" {
		return
	}
	trippedLabelsMap := make(map[string]string)
	if err := json.Unmarshal([]byte(trippedLabels), &trippedLabelsMap); err != nil {
		return
	}
	for k, v := range trippedLabelsMap {
		labels[k] = v
	}
}

func recoverSelectors(pod *corev1.Pod, cns *ClustersNodeSelection) {
	if cns != nil {
		pod.Spec.NodeSelector = cns.NodeSelector
		pod.Spec.Tolerations = cns.Tolerations
		if pod.Spec.Affinity == nil {
			pod.Spec.Affinity = cns.Affinity
		} else {
			if cns.Affinity != nil && cns.Affinity.NodeAffinity != nil {
				if pod.Spec.Affinity.NodeAffinity != nil {
					pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = cns.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
				} else {
					pod.Spec.Affinity.NodeAffinity = cns.Affinity.NodeAffinity
				}
			} else {
				pod.Spec.Affinity.NodeAffinity = nil
			}
		}
	} else {
		pod.Spec.NodeSelector = nil
		pod.Spec.Tolerations = nil
		if pod.Spec.Affinity != nil && pod.Spec.Affinity.NodeAffinity != nil {
			pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = nil
		}
	}
	if pod.Spec.Affinity != nil {
		if pod.Spec.Affinity.NodeAffinity != nil {
			if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil &&
				pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution == nil {
				pod.Spec.Affinity.NodeAffinity = nil
			}
		}
		if pod.Spec.Affinity.NodeAffinity == nil && pod.Spec.Affinity.PodAffinity == nil &&
			pod.Spec.Affinity.PodAntiAffinity == nil {
			pod.Spec.Affinity = nil
		}
	}
}

func TrimLabels(labels map[string]string, ignoreLabels []string) map[string]string {
	if ignoreLabels == nil {
		return nil
	}
	trippedLabels := make(map[string]string, len(ignoreLabels))
	for _, key := range ignoreLabels {
		if labels[key] == "" {
			continue
		}
		trippedLabels[key] = labels[key]
		delete(labels, key)
	}
	return trippedLabels
}

func ConvertAnnotations(annotation map[string]string) *ClustersNodeSelection {
	if annotation == nil {
		return nil
	}
	val := annotation[SelectorKey]
	if len(val) == 0 {
		return nil
	}

	var cns ClustersNodeSelection
	err := json.Unmarshal([]byte(val), &cns)
	if err != nil {
		return nil
	}
	return &cns
}
