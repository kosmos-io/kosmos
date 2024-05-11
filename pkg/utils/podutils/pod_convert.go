package podutils

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

// ConvertPod perform all conversions
func ConvertPod(pod *corev1.Pod, cpcp []kosmosv1alpha1.ClusterPodConvertPolicy, pcp []kosmosv1alpha1.PodConvertPolicy) {
	if len(cpcp) <= 0 && len(pcp) <= 0 {
		return
	}

	var clusterScopeChoose *kosmosv1alpha1.ClusterPodConvertPolicy
	var nsScopeChoose *kosmosv1alpha1.PodConvertPolicy
	var converters *kosmosv1alpha1.Converters
	if len(cpcp) > 0 {
		// current, use the first non-empty matching policy
		for idx, po := range cpcp {
			if po.Spec.Converters != nil {
				clusterScopeChoose = &cpcp[idx]
				break
			}
		}
		if clusterScopeChoose == nil {
			return
		}
		converters = clusterScopeChoose.Spec.Converters
		pod.Annotations[utils.KosmosConvertLabels] = clusterScopeChoose.Name
		klog.V(4).Infof("Convert pod %v/%+v, policy: %s", pod.Namespace, pod.Name, clusterScopeChoose.Name)
	} else {
		// current, use the first non-empty matching policy
		for idx, po := range pcp {
			if po.Spec.Converters != nil {
				nsScopeChoose = &pcp[idx]
				break
			}
		}
		if nsScopeChoose == nil {
			return
		}
		converters = nsScopeChoose.Spec.Converters
		pod.Annotations[utils.KosmosConvertLabels] = nsScopeChoose.Name
		klog.V(4).Infof("Convert pod %v/%+v, policy: %s", pod.Namespace, pod.Name, nsScopeChoose.Name)
	}

	convertSchedulerName(pod, converters.SchedulerNameConverter)
	convertNodeName(pod, converters.NodeNameConverter)
	convertNodeSelector(pod, converters.NodeSelectorConverter)
	converToleration(pod, converters.TolerationConverter)
	convertAffinity(pod, converters.AffinityConverter)
	convertTopologySpreadConstraints(pod, converters.TopologySpreadConstraintsConverter)
	convertHostAliases(pod, converters.HostAliasesConverter)
}

func convertSchedulerName(pod *corev1.Pod, converter *kosmosv1alpha1.SchedulerNameConverter) {
	if converter == nil {
		return
	}

	switch converter.ConvertType {
	case kosmosv1alpha1.Add:
		if pod.Spec.SchedulerName == "" {
			pod.Spec.SchedulerName = converter.SchedulerName
		}
	case kosmosv1alpha1.Remove:
		pod.Spec.SchedulerName = ""
	case kosmosv1alpha1.Replace:
		pod.Spec.SchedulerName = converter.SchedulerName
	default:
		klog.Warningf("Skip other convert type, SchedulerName: %s", converter.ConvertType)
	}
}

func convertNodeName(pod *corev1.Pod, converter *kosmosv1alpha1.NodeNameConverter) {
	if converter == nil {
		return
	}

	switch converter.ConvertType {
	case kosmosv1alpha1.Add:
		if pod.Spec.NodeName == "" {
			pod.Spec.NodeName = converter.NodeName
		}
	case kosmosv1alpha1.Remove:
		pod.Spec.NodeName = ""
	case kosmosv1alpha1.Replace:
		pod.Spec.NodeName = converter.NodeName
	default:
		klog.Warningf("Skip other convert type, NodeName: %s", converter.ConvertType)
	}
}

func converToleration(pod *corev1.Pod, conveter *kosmosv1alpha1.TolerationConverter) {
	if conveter == nil {
		return
	}

	switch conveter.ConvertType {
	case kosmosv1alpha1.Add:
		if pod.Spec.Tolerations == nil {
			pod.Spec.Tolerations = conveter.Tolerations
		}
	case kosmosv1alpha1.Remove:
		pod.Spec.Tolerations = nil
	case kosmosv1alpha1.Replace:
		pod.Spec.Tolerations = conveter.Tolerations
	default:
		klog.Warningf("Skip other convert type, Tolerations: %s", conveter.ConvertType)
	}
}

func convertNodeSelector(pod *corev1.Pod, converter *kosmosv1alpha1.NodeSelectorConverter) {
	if converter == nil {
		return
	}

	switch converter.ConvertType {
	case kosmosv1alpha1.Add:
		if pod.Spec.NodeSelector == nil {
			pod.Spec.NodeSelector = converter.NodeSelector
		}
	case kosmosv1alpha1.Remove:
		pod.Spec.NodeSelector = nil
	case kosmosv1alpha1.Replace:
		pod.Spec.NodeSelector = converter.NodeSelector
	default:
		klog.Warningf("Skip other convert type, NodeSelector: %s", converter.ConvertType)
	}
}

func convertAffinity(pod *corev1.Pod, converter *kosmosv1alpha1.AffinityConverter) {
	if converter == nil {
		return
	}

	switch converter.ConvertType {
	case kosmosv1alpha1.Add:
		if pod.Spec.Affinity == nil {
			pod.Spec.Affinity = converter.Affinity
		}
	case kosmosv1alpha1.Remove:
		pod.Spec.Affinity = nil
	case kosmosv1alpha1.Replace:
		pod.Spec.Affinity = converter.Affinity
	default:
		klog.Warningf("Skip other convert type, Affinity: %s", converter.ConvertType)
	}
}

func convertTopologySpreadConstraints(pod *corev1.Pod, converter *kosmosv1alpha1.TopologySpreadConstraintsConverter) {
	if converter == nil {
		return
	}

	switch converter.ConvertType {
	case kosmosv1alpha1.Add:
		if pod.Spec.Affinity == nil {
			pod.Spec.TopologySpreadConstraints = converter.TopologySpreadConstraints
		}
	case kosmosv1alpha1.Remove:
		pod.Spec.TopologySpreadConstraints = nil
	case kosmosv1alpha1.Replace:
		pod.Spec.TopologySpreadConstraints = converter.TopologySpreadConstraints
	default:
		klog.Warningf("Skip other convert type, TopologySpreadConstraints: %s", converter.ConvertType)
	}
}

func convertHostAliases(pod *corev1.Pod, converter *kosmosv1alpha1.HostAliasesConverter) {
	if converter == nil {
		return
	}

	switch converter.ConvertType {
	case kosmosv1alpha1.Add:
		if pod.Spec.HostAliases == nil {
			pod.Spec.HostAliases = converter.HostAliases
		}
	case kosmosv1alpha1.Remove:
		pod.Spec.HostAliases = nil
	case kosmosv1alpha1.Replace:
		pod.Spec.HostAliases = converter.HostAliases
	default:
		klog.Warningf("Skip other convert type, HostAliases: %s", converter.ConvertType)
	}
}
