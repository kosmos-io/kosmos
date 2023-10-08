package helper

import (
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddEndpointSliceLabel adds label for the given endpointSlice.
func AddEndpointSliceLabel(eps *discoveryv1.EndpointSlice, labelKey string, labelValue string) {
	epsLabels := eps.GetLabels()
	if epsLabels == nil {
		epsLabels = make(map[string]string, 1)
	}
	epsLabels[labelKey] = labelValue
	eps.SetLabels(epsLabels)
}

// GetLabelOrAnnotationValue get the value by labelKey, otherwise returns an empty string.
func GetLabelOrAnnotationValue(values map[string]string, valueKey string) string {
	if values == nil {
		return ""
	}

	return values[valueKey]
}

// RemoveLabel removes the label from the given endpointSlice.
func RemoveLabel(eps *discoveryv1.EndpointSlice, labelKey string) {
	labels := eps.GetLabels()
	delete(labels, labelKey)
	eps.SetLabels(labels)
}

// HasLabel returns if a ObjectMeta has a key
func HasLabel(m metav1.ObjectMeta, labelKey string) bool {
	objLabels := m.GetLabels()
	if objLabels == nil {
		return false
	}
	if _, exists := objLabels[labelKey]; exists {
		return true
	} else {
		return false
	}
}
