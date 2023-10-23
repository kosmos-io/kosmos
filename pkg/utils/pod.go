package utils

import (
	"encoding/json"
	"strings"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"
)

func GetSecrets(pod *corev1.Pod) []string {
	secretNames := []string{}
	for _, v := range pod.Spec.Volumes {
		switch {
		case v.Secret != nil:
			if strings.HasPrefix(v.Name, "default-token") {
				continue
			}
			klog.Infof("pod %s depends on secret %s", pod.Name, v.Secret.SecretName)
			secretNames = append(secretNames, v.Secret.SecretName)

		case v.CephFS != nil:
			klog.Infof("pod %s depends on secret %s", pod.Name, v.CephFS.SecretRef.Name)
			secretNames = append(secretNames, v.CephFS.SecretRef.Name)
		case v.Cinder != nil:
			klog.Infof("pod %s depends on secret %s", pod.Name, v.Cinder.SecretRef.Name)
			secretNames = append(secretNames, v.Cinder.SecretRef.Name)
		case v.RBD != nil:
			klog.Infof("pod %s depends on secret %s", pod.Name, v.RBD.SecretRef.Name)
			secretNames = append(secretNames, v.RBD.SecretRef.Name)
		default:
			klog.Warning("Skip other type volumes")
		}
	}
	if pod.Spec.ImagePullSecrets != nil {
		for _, s := range pod.Spec.ImagePullSecrets {
			secretNames = append(secretNames, s.Name)
		}
	}
	klog.Infof("pod %s depends on secrets %s", pod.Name, secretNames)
	return secretNames
}

func GetConfigmaps(pod *corev1.Pod) []string {
	cmNames := []string{}
	for _, v := range pod.Spec.Volumes {
		if v.ConfigMap == nil {
			continue
		}
		cmNames = append(cmNames, v.ConfigMap.Name)
	}
	klog.Infof("pod %s depends on configMap %s", pod.Name, cmNames)
	return cmNames
}

func GetPVCs(pod *corev1.Pod) []string {
	cmNames := []string{}
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim == nil {
			continue
		}
		cmNames = append(cmNames, v.PersistentVolumeClaim.ClaimName)
	}
	klog.Infof("pod %s depends on pvc %v", pod.Name, cmNames)
	return cmNames
}

func SetObjectGlobal(obj *metav1.ObjectMeta) {
	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	obj.Annotations[KosmosGlobalLabel] = "true"
}

func SetUnstructuredObjGlobal(unstructuredObj *unstructured.Unstructured) {
	annotationsMap := unstructuredObj.GetAnnotations()
	if annotationsMap == nil {
		annotationsMap = map[string]string{}
	}
	annotationsMap[KosmosGlobalLabel] = "true"

	unstructuredObj.SetAnnotations(annotationsMap)
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

func IsEqual(pod1, pod2 *corev1.Pod) bool {
	return cmp.Equal(pod1.Spec.Containers, pod2.Spec.Containers) &&
		cmp.Equal(pod1.Spec.InitContainers, pod2.Spec.InitContainers) &&
		cmp.Equal(pod1.Spec.ActiveDeadlineSeconds, pod2.Spec.ActiveDeadlineSeconds) &&
		cmp.Equal(pod1.Spec.Tolerations, pod2.Spec.Tolerations) &&
		cmp.Equal(pod1.ObjectMeta.Labels, pod2.Labels) &&
		cmp.Equal(pod1.ObjectMeta.Annotations, pod2.Annotations)
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

func FitObjectMeta(meta *metav1.ObjectMeta) {
	meta.UID = ""
	meta.ResourceVersion = ""
	// meta.SelfLink = ""
	meta.OwnerReferences = nil
}

func FitUnstructuredObjMeta(unstructuredObj *unstructured.Unstructured) {
	unstructuredObj.SetUID("")
	unstructuredObj.SetResourceVersion("")
	unstructuredObj.SetOwnerReferences(nil)
}

func FitPod(pod *corev1.Pod, ignoreLabels []string) *corev1.Pod {
	vols := []corev1.Volume{}
	for _, v := range pod.Spec.Volumes {
		if strings.HasPrefix(v.Name, "default-token") {
			continue
		}
		vols = append(vols, v)
	}

	podCopy := pod.DeepCopy()
	FitObjectMeta(&podCopy.ObjectMeta)
	if podCopy.Labels == nil {
		podCopy.Labels = make(map[string]string)
	}
	if podCopy.Annotations == nil {
		podCopy.Annotations = make(map[string]string)
	}
	podCopy.Labels[KosmosPodLabel] = "true"
	cns := ConvertAnnotations(pod.Annotations)
	recoverSelectors(podCopy, cns)
	podCopy.Spec.Containers = fitContainers(pod.Spec.Containers)
	podCopy.Spec.InitContainers = fitContainers(pod.Spec.InitContainers)
	podCopy.Spec.Volumes = vols
	podCopy.Spec.NodeName = ""
	podCopy.Status = corev1.PodStatus{}

	podCopy.Spec.SchedulerName = ""

	tripped := FitLabels(podCopy.ObjectMeta.Labels, ignoreLabels)
	if tripped != nil {
		trippedStr, err := json.Marshal(tripped)
		if err != nil {
			return podCopy
		}
		podCopy.Annotations[KosmosTrippedLabels] = string(trippedStr)
	}

	return podCopy
}

func fitContainers(containers []corev1.Container) []corev1.Container {
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

func IsKosmosPod(pod *corev1.Pod) bool {
	if pod.Labels != nil && pod.Labels[KosmosPodLabel] == "true" {
		return true
	}
	return false
}

func RecoverLabels(labels map[string]string, annotations map[string]string) {
	trippedLabels := annotations[KosmosTrippedLabels]
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

func FitLabels(labels map[string]string, ignoreLabels []string) map[string]string {
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
	if orig.Annotations[KosmosSelectorKey] != update.Annotations[KosmosSelectorKey] {
		if cns := ConvertAnnotations(update.Annotations); cns != nil {
			orig.Spec.Tolerations = cns.Tolerations
		}
	}
	orig.Labels = update.Labels
	orig.Annotations = update.Annotations
	orig.Spec.ActiveDeadlineSeconds = update.Spec.ActiveDeadlineSeconds
	if orig.Labels != nil {
		FitLabels(orig.ObjectMeta.Labels, ignoreLabels)
	}
}

func ConvertAnnotations(annotation map[string]string) *ClustersNodeSelection {
	if annotation == nil {
		return nil
	}
	val := annotation[KosmosSelectorKey]
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
