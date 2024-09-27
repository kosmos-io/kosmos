// nolint:dupl
package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

// TestIsPVEqual tests IsPVEqual function
func TestIsPVEqual(t *testing.T) {
	tests := []struct {
		name   string
		pv     *corev1.PersistentVolume
		clone  *corev1.PersistentVolume
		expect bool
	}{
		{
			name: "Equal PVs",
			pv: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"key": "value",
					},
				},
				Spec:   corev1.PersistentVolumeSpec{},
				Status: corev1.PersistentVolumeStatus{},
			},
			clone: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"key": "value",
					},
				},
				Spec:   corev1.PersistentVolumeSpec{},
				Status: corev1.PersistentVolumeStatus{},
			},
			expect: true,
		},
		{
			name: "Different Spec",
			pv: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"key": "value",
					},
				},
				Spec:   corev1.PersistentVolumeSpec{Capacity: corev1.ResourceList{"storage": resource.MustParse("10Gi")}},
				Status: corev1.PersistentVolumeStatus{},
			},
			clone: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"key": "value",
					},
				},
				Spec:   corev1.PersistentVolumeSpec{Capacity: corev1.ResourceList{"storage": resource.MustParse("20Gi")}},
				Status: corev1.PersistentVolumeStatus{},
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPVEqual(tt.pv, tt.clone)
			if result != tt.expect {
				t.Errorf("expected %v, got %v", tt.expect, result)
			}
		})
	}
}

// TestIsOne2OneMode tests the IsOne2OneMode function
func TestIsOne2OneMode(t *testing.T) {
	leafModels := []kosmosv1alpha1.LeafModel{}
	tests := []struct {
		name     string
		cluster  *kosmosv1alpha1.Cluster
		expected bool
	}{
		{
			name: "One-to-One Mode",
			cluster: &kosmosv1alpha1.Cluster{
				Spec: kosmosv1alpha1.ClusterSpec{
					ClusterTreeOptions: &kosmosv1alpha1.ClusterTreeOptions{
						LeafModels: leafModels,
					},
				},
			},
			expected: true,
		},
		{
			name: "Not One-to-One Mode",
			cluster: &kosmosv1alpha1.Cluster{
				Spec: kosmosv1alpha1.ClusterSpec{
					ClusterTreeOptions: &kosmosv1alpha1.ClusterTreeOptions{
						LeafModels: nil,
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOne2OneMode(tt.cluster)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsPVCEqual tests the IsPVCEqual function.
func TestIsPVCEqual(t *testing.T) {
	tests := []struct {
		name   string
		pvc    *corev1.PersistentVolumeClaim
		clone  *corev1.PersistentVolumeClaim
		expect bool
	}{
		{
			name: "Equal PVCs",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"key": "value",
					},
				},
				Spec:   corev1.PersistentVolumeClaimSpec{},
				Status: corev1.PersistentVolumeClaimStatus{},
			},
			clone: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"key": "value",
					},
				},
				Spec:   corev1.PersistentVolumeClaimSpec{},
				Status: corev1.PersistentVolumeClaimStatus{},
			},
			expect: true,
		},
		{
			name: "Different Spec",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"key": "value",
					},
				},
				Spec:   corev1.PersistentVolumeClaimSpec{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{"storage": resource.MustParse("10Gi")}}},
				Status: corev1.PersistentVolumeClaimStatus{},
			},
			clone: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"key": "value",
					},
				},
				Spec:   corev1.PersistentVolumeClaimSpec{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{"storage": resource.MustParse("20Gi")}}},
				Status: corev1.PersistentVolumeClaimStatus{},
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPVCEqual(tt.pvc, tt.clone)
			if result != tt.expect {
				t.Errorf("expected %v, got %v", tt.expect, result)
			}
		})
	}
}
