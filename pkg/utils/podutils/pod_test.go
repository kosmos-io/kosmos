// nolint:dupl
package podutils

import (
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

// 辅助函数，用于比较两个字符串切片是否相等.
func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]int)
	for _, v := range a {
		m[v]++
	}
	for _, v := range b {
		m[v]--
		if m[v] < 0 {
			return false
		}
	}
	return true
}

// TestGetSecrets tests the GetSecrets function.
// nolint:gosec
func TestGetSecrets(t *testing.T) {
	tests := []struct {
		name                     string
		pod                      corev1.Pod
		expectedSecrets          []string
		expectedImagePullSecrets []string
	}{
		{
			name: "No volumes or image pull secrets",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes:          nil,
					ImagePullSecrets: nil,
				},
			},
			expectedSecrets:          []string{},
			expectedImagePullSecrets: []string{},
		},
		{
			name: "Single secret volume",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "my-secret-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "my-secret",
								},
							},
						},
					},
					ImagePullSecrets: nil,
				},
			},
			expectedSecrets:          []string{"my-secret"},
			expectedImagePullSecrets: []string{},
		},
		{
			name: "Multiple secret volumes and image pull secrets",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "ceph-volume",
							VolumeSource: corev1.VolumeSource{
								CephFS: &corev1.CephFSVolumeSource{
									SecretRef: &corev1.LocalObjectReference{
										Name: "ceph-secret",
									},
								},
							},
						},
						{
							Name: "cinder-volume",
							VolumeSource: corev1.VolumeSource{
								Cinder: &corev1.CinderVolumeSource{
									SecretRef: &corev1.LocalObjectReference{
										Name: "cinder-secret",
									},
								},
							},
						},
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "image-pull-secret"},
					},
				},
			},
			expectedSecrets:          []string{"ceph-secret", "cinder-secret"},
			expectedImagePullSecrets: []string{"image-pull-secret"},
		},
		{
			name: "Volume with unsupported type",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "unsupported-volume",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/path/to/host",
								},
							},
						},
					},
					ImagePullSecrets: nil,
				},
			},
			expectedSecrets:          []string{},
			expectedImagePullSecrets: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清理 klog 输出
			klog.SetOutput(nil)

			secretNames, imagePullSecrets := GetSecrets(&tt.pod)

			if !equal(secretNames, tt.expectedSecrets) {
				t.Errorf("expected secrets %v, got %v", tt.expectedSecrets, secretNames)
			}
			if !equal(imagePullSecrets, tt.expectedImagePullSecrets) {
				t.Errorf("expected image pull secrets %v, got %v", tt.expectedImagePullSecrets, imagePullSecrets)
			}
		})
	}
}

// TestGetConfigmaps tests the GetConfigmaps function.
// nolint:gosec
func TestGetConfigmaps(t *testing.T) {
	tests := []struct {
		name        string
		pod         corev1.Pod
		expectedCMs []string
	}{
		{
			name: "No config maps",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: nil,
				},
			},
			expectedCMs: []string{},
		},
		{
			name: "Single config map volume",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "my-config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "my-config",
									},
								},
							},
						},
					},
				},
			},
			expectedCMs: []string{"my-config"},
		},
		{
			name: "Multiple config map volumes",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "config-volume-1",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "config-1",
									},
								},
							},
						},
						{
							Name: "config-volume-2",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "config-2",
									},
								},
							},
						},
					},
				},
			},
			expectedCMs: []string{"config-1", "config-2"},
		},
		{
			name: "Change root CA config map name",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "root-ca-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: utils.RooTCAConfigMapName,
									},
								},
							},
						},
					},
				},
			},
			expectedCMs: []string{utils.RooTCAConfigMapName},
		},
	}

	for _, ttt := range tests {
		t.Run(ttt.name, func(t *testing.T) {
			// 清理 klog 输出
			klog.SetOutput(nil)

			cmNames := GetConfigmaps(&ttt.pod)

			if !equal(cmNames, ttt.expectedCMs) {
				t.Errorf("expected config maps %v, got %v", ttt.expectedCMs, cmNames)
			}
		})
	}
}

// TestGetPVCs tests the GetPVCs function.
// nolint:gosec
func TestGetPVCs(t *testing.T) {
	tests := []struct {
		name         string
		pod          corev1.Pod
		expectedPVCs []string
	}{
		{
			name: "No PVCs",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: nil,
				},
			},
			expectedPVCs: []string{},
		},
		{
			name: "Single PVC volume",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "my-pvc-volume",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "my-pvc",
								},
							},
						},
					},
				},
			},
			expectedPVCs: []string{"my-pvc"},
		},
		{
			name: "Multiple PVC volumes",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "pvc-volume-1",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc-1",
								},
							},
						},
						{
							Name: "pvc-volume-2",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc-2",
								},
							},
						},
					},
				},
			},
			expectedPVCs: []string{"pvc-1", "pvc-2"},
		},
		{
			name: "Mix of PVCs and other volumes",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "my-pvc-volume",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "my-pvc",
								},
							},
						},
						{
							Name: "unsupported-volume",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/path/to/host",
								},
							},
						},
					},
				},
			},
			expectedPVCs: []string{"my-pvc"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// 清理 klog 输出
			klog.SetOutput(nil)

			pvcNames := GetPVCs(&test.pod)

			if !equal(pvcNames, test.expectedPVCs) {
				t.Errorf("expected PVCs %v, got %v", test.expectedPVCs, pvcNames)
			}
		})
	}
}

// TestSetObjectGlobal tests the SetObjectGlobal function.
func TestSetObjectGlobal(t *testing.T) {
	tests := []struct {
		name     string
		input    *metav1.ObjectMeta
		expected map[string]string
	}{
		{
			name:     "Set global label on empty annotations",
			input:    &metav1.ObjectMeta{},
			expected: map[string]string{utils.KosmosGlobalLabel: "true"},
		},
		{
			name: "Set global label on existing annotations",
			input: &metav1.ObjectMeta{
				Annotations: map[string]string{"existing": "value"},
			},
			expected: map[string]string{
				"existing":              "value",
				utils.KosmosGlobalLabel: "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetObjectGlobal(tt.input)
			if tt.input.Annotations[utils.KosmosGlobalLabel] != "true" {
				t.Errorf("expected annotation %s to be true", utils.KosmosGlobalLabel)
			}
			for key, value := range tt.expected {
				if tt.input.Annotations[key] != value {
					t.Errorf("expected annotation %s to be %s, got %s", key, value, tt.input.Annotations[key])
				}
			}
		})
	}
}

// TestSetUnstructuredObjGlobal tests the SetUnstructuredObjGlobal function
func TestSetUnstructuredObjGlobal(t *testing.T) {
	tests := []struct {
		name     string
		input    *unstructured.Unstructured
		expected map[string]string
	}{
		{
			name:  "Set global label on empty annotations",
			input: &unstructured.Unstructured{},
			expected: map[string]string{
				utils.KosmosGlobalLabel: "true",
			},
		},
		{
			name: "Set global label on existing annotations",
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"existing": "value",
						},
					},
				},
			},
			expected: map[string]string{
				"existing":              "value",
				utils.KosmosGlobalLabel: "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetUnstructuredObjGlobal(tt.input)
			annotations := tt.input.GetAnnotations()
			if annotations[utils.KosmosGlobalLabel] != "true" {
				t.Errorf("expected annotation %s to be true", utils.KosmosGlobalLabel)
			}
			for key, value := range tt.expected {
				if annotations[key] != value {
					t.Errorf("expected annotation %s to be %s, got %s", key, value, annotations[key])
				}
			}
		})
	}
}

// 辅助函数，用于返回指向 int64 的指针
func int64Ptr(i int64) *int64 {
	return &i
}

// TestDeleteGraceTimeEqual tests the DeleteGraceTimeEqual function.
func TestDeleteGraceTimeEqual(t *testing.T) {
	tests := []struct {
		name     string
		old      *int64
		new      *int64
		expected bool
	}{
		{
			name:     "Both nil",
			old:      nil,
			new:      nil,
			expected: true,
		},
		{
			name:     "Both equal",
			old:      int64Ptr(5),
			new:      int64Ptr(5),
			expected: true,
		},
		{
			name:     "Old is nil, new is not",
			old:      nil,
			new:      int64Ptr(5),
			expected: false,
		},
		{
			name:     "Old is not nil, new is nil",
			old:      int64Ptr(5),
			new:      nil,
			expected: false,
		},
		{
			name:     "Both not equal",
			old:      int64Ptr(5),
			new:      int64Ptr(10),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeleteGraceTimeEqual(tt.old, tt.new)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsEqual tests the IsEqual function
func TestIsEqual(t *testing.T) {
	tests := []struct {
		name     string
		pod1     *corev1.Pod
		pod2     *corev1.Pod
		expected bool
	}{
		{
			name: "Equal pods",
			pod1: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": "test"},
					Annotations: map[string]string{"annotation": "value"},
				},
			},
			pod2: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": "test"},
					Annotations: map[string]string{"annotation": "value"},
				},
			},
			expected: true,
		},
		{
			name: "Different containers",
			pod1: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
			},
			pod2: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container2"}},
				},
			},
			expected: false,
		},
		{
			name: "Different labels",
			pod1: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
			},
			pod2: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "different"},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEqual(tt.pod1, tt.pod2)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestShouldEnqueue tests the ShouldEnqueue function.
func TestShouldEnqueue(t *testing.T) {
	tests := []struct {
		name     string
		oldPod   *corev1.Pod
		newPod   *corev1.Pod
		expected bool
	}{
		{
			name: "Pods are equal",
			oldPod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
			},
			newPod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
			},
			expected: false,
		},
		{
			name: "Different containers",
			oldPod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
			},
			newPod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container2"}},
				},
			},
			expected: true,
		},
		{
			name: "Different deletion grace period",
			oldPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					DeletionGracePeriodSeconds: int64Ptr(30),
				},
			},
			newPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					DeletionGracePeriodSeconds: int64Ptr(60),
				},
			},
			expected: true,
		},
		{
			name: "Different deletion timestamps",
			oldPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{
						Time: time.Now(),
					},
				},
			},
			newPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{
						Time: time.Now().Add(1 * time.Minute),
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldEnqueue(tt.oldPod, tt.newPod)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestFitObjectMeta tests the FitObjectMeta function.
func TestFitObjectMeta(t *testing.T) {
	tests := []struct {
		name     string
		input    *metav1.ObjectMeta
		expected metav1.ObjectMeta
	}{
		{
			name: "Clear UID and ResourceVersion",
			input: &metav1.ObjectMeta{
				UID:             "12345",
				ResourceVersion: "1",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name: "owner",
					},
				},
			},
			expected: metav1.ObjectMeta{
				UID:             "",
				ResourceVersion: "",
				OwnerReferences: nil,
			},
		},
		{
			name: "No changes needed",
			input: &metav1.ObjectMeta{
				UID:             "",
				ResourceVersion: "",
				OwnerReferences: nil,
			},
			expected: metav1.ObjectMeta{
				UID:             "",
				ResourceVersion: "",
				OwnerReferences: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			FitObjectMeta(tt.input)
			if !reflect.DeepEqual(*tt.input, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, *tt.input)
			}
		})
	}
}
