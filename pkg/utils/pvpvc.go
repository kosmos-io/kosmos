package utils

import (
	"fmt"
	"reflect"

	v1 "k8s.io/api/core/v1"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

func IsPVEqual(pv *v1.PersistentVolume, clone *v1.PersistentVolume) bool {
	if reflect.DeepEqual(pv.Annotations, clone.Annotations) &&
		reflect.DeepEqual(pv.Spec, clone.Spec) &&
		reflect.DeepEqual(pv.Status, clone.Status) {
		return true
	}
	return false
}

func IsOne2OneMode(cluster *kosmosv1alpha1.Cluster) bool {
	return cluster.Spec.ClusterTreeOptions.LeafModels != nil
}

func NodeAffinity4RootPV(pv *v1.PersistentVolume, isOne2OneMode bool, clusterName string) string {
	node4RootPV := fmt.Sprintf("%s%s", KosmosNodePrefix, clusterName)
	if isOne2OneMode {
		for _, v := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
			for _, val := range v.MatchFields {
				if val.Key == NodeHostnameValue || val.Key == NodeHostnameValueBeta || val.Key == OpenebsPVNodeLabel {
					node4RootPV = val.Values[0]
				}
			}
			for _, val := range v.MatchExpressions {
				if val.Key == NodeHostnameValue || val.Key == NodeHostnameValueBeta || val.Key == OpenebsPVNodeLabel {
					node4RootPV = val.Values[0]
				}
			}
		}
	}
	return node4RootPV
}

func IsPVCEqual(pvc *v1.PersistentVolumeClaim, clone *v1.PersistentVolumeClaim) bool {
	if reflect.DeepEqual(pvc.Annotations, clone.Annotations) &&
		reflect.DeepEqual(pvc.Spec, clone.Spec) &&
		reflect.DeepEqual(pvc.Status, clone.Status) {
		return true
	}
	return false
}
