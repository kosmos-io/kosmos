package helper

import (
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddEndpointSliceAnnotation adds annotation for the given endpointSlice.
func AddEndpointSliceAnnotation(eps *discoveryv1.EndpointSlice, annotationKey string, annotationValue string) {
	epsAnnotations := eps.GetAnnotations()
	if epsAnnotations == nil {
		epsAnnotations = make(map[string]string, 1)
	}
	epsAnnotations[annotationKey] = annotationValue
	eps.SetAnnotations(epsAnnotations)
}

// AddEndpointSliceLabel adds label for the given endpointSlice.
func AddEndpointSliceLabel(eps *discoveryv1.EndpointSlice, labelKey string, labelValue string) {
	epsLabel := eps.GetLabels()
	if epsLabel == nil {
		epsLabel = make(map[string]string, 1)
	}
	epsLabel[labelKey] = labelValue
	eps.SetLabels(epsLabel)
}

// GetLabelOrAnnotationValue get the value by labelKey, otherwise returns an empty string.
func GetLabelOrAnnotationValue(values map[string]string, valueKey string) string {
	if values == nil {
		return ""
	}

	return values[valueKey]
}

// RemoveAnnotation removes the label from the given endpointSlice.
func RemoveAnnotation(eps *discoveryv1.EndpointSlice, annotationKey string) {
	annotations := eps.GetAnnotations()
	delete(annotations, annotationKey)
	eps.SetAnnotations(annotations)
}

// HasAnnotation returns if a ObjectMeta has a key
func HasAnnotation(m metav1.ObjectMeta, key string) bool {
	annotations := m.GetAnnotations()
	if annotations == nil {
		return false
	}
	if _, exists := annotations[key]; exists {
		return true
	} else {
		return false
	}
}
