// nolint:dupl
package utils

import (
	"encoding/json"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// jsonEqual is a helper function to compare JSON byte slices
func jsonEqual(a, b []byte) bool {
	var aJSON, bJSON interface{}
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	err := json.Unmarshal(a, &aJSON)
	if err != nil {
		return false
	}

	err = json.Unmarshal(b, &bJSON)
	if err != nil {
		return false
	}
	return reflect.DeepEqual(aJSON, bJSON)
}

// TestCreateMergePatch tests the CreateMergePatch function.
func TestCreateMergePatch(t *testing.T) {
	tests := []struct {
		name     string
		original interface{}
		new      interface{}
		expected []byte
	}{
		{
			name: "Same objects",
			original: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			new: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			expected: []byte(`{}`),
		},
		{
			name: "Different objects",
			original: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			new: map[string]interface{}{
				"key1": "value1",
				"key2": "value3",
			},
			expected: []byte(`{"key2":"value3"}`),
		},
		{
			name: "Add new key",
			original: map[string]interface{}{
				"key1": "value1",
			},
			new: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			expected: []byte(`{"key2":"value2"}`),
		},
		{
			name: "Remove key",
			original: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			new: map[string]interface{}{
				"key1": "value1",
			},
			expected: []byte(`{"key2":null}`),
		},
		{
			name: "Nested objects",
			original: map[string]interface{}{
				"key1": map[string]interface{}{
					"subkey": "value1",
				},
			},
			new: map[string]interface{}{"key1": map[string]interface{}{
				"subkey": "value2",
			},
			},
			expected: []byte(`{"key1":{"subkey":"value2"}}`),
		},
		{
			name:     "Empty original and new",
			original: map[string]interface{}{},
			new:      map[string]interface{}{},
			expected: []byte(`{}`),
		},
		{
			name:     "Empty original with non-empty new",
			original: map[string]interface{}{},
			new: map[string]interface{}{
				"key1": "value1",
			},
			expected: []byte(`{"key1":"value1"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateMergePatch(tt.original, tt.new)
			if err != nil {
				t.Fatalf("CreateMergePatch() error = %v", err)
			}
			if !jsonEqual(got, tt.expected) {
				t.Errorf("CreateMergePatch() got = %s, want %s", got, tt.expected)
			}
		})
	}
}

// TestIsKosmosNode tests the IsKosmosNode function.
func TestIsKosmosNode(t *testing.T) {
	tests := []struct {
		name     string
		node     *corev1.Node
		expected bool
	}{
		{
			name:     "nil node",
			node:     nil,
			expected: false,
		},
		{
			name: "node without label",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			expected: false,
		},
		{
			name: "node with incorrect label value",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						KosmosNodeLabel: "false",
					},
				},
			},
			expected: false,
		},
		{
			name: "node with correct label value",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						KosmosNodeLabel: KosmosNodeValue,
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsKosmosNode(tt.node)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsExcludeNode tests the IsExcludeNode function.
func TestIsExcludeNode(t *testing.T) {
	tests := []struct {
		name     string
		node     *corev1.Node
		expected bool
	}{
		{
			name:     "nil node",
			node:     nil,
			expected: false,
		},
		{
			name: "node without label",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			expected: false,
		},
		{
			name: "node with incorrect label value",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						KosmosExcludeNodeLabel: "false",
					},
				},
			},
			expected: false,
		},
		{
			name: "node with correct label value",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						KosmosExcludeNodeLabel: KosmosExcludeNodeValue,
					},
				},
			},
			expected: true,
		},
		{
			name: "node with multiple labels",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						KosmosExcludeNodeLabel: KosmosExcludeNodeValue,
						"another-label":        "some-value",
					},
				},
			},
			expected: true,
		},
		{
			name: "node with label but different key",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"some-other-label": KosmosExcludeNodeValue,
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsExcludeNode(tt.node)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsVirtualPod tests the IsVirtualPod function.
func TestIsVirtualPod(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod without labels",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			expected: false,
		},
		{
			name: "pod with incorrect label value",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						KosmosPodLabel: "false",
					},
				},
			},
			expected: false,
		},
		{
			name: "pod with correct label value",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						KosmosPodLabel: "true",
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with multiple labels",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						KosmosPodLabel:  "true",
						"another-label": "some-value",
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsVirtualPod(tt.pod)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Helper function to compare two ConfigMaps
func equalConfigMaps(a, b *corev1.ConfigMap) bool {
	if len(a.Labels) != len(b.Labels) || len(a.Data) != len(b.Data) {
		return false
	}

	for k, v := range a.Labels {
		if b.Labels[k] != v {
			return false
		}
	}

	for k, v := range a.Data {
		if b.Data[k] != v {
			return false
		}
	}

	return true
}

// TestUpdateConfigMap tests the update of a configmap
func TestUpdateConfigMap(t *testing.T) {
	tests := []struct {
		name     string
		old      *corev1.ConfigMap
		new      *corev1.ConfigMap
		expected *corev1.ConfigMap
	}{
		{
			name: "update with new labels and data",
			old: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"old-label": "value",
					},
				},
				Data: map[string]string{
					"old-key": "old-value",
				},
			},
			new: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"new-label": "value",
					},
				},
				Data: map[string]string{
					"new-key": "new-value",
				},
			},
			expected: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"new-label": "value",
					},
				},
				Data: map[string]string{
					"new-key": "new-value",
				},
			},
		},
		{
			name: "update with empty new ConfigMap",
			old: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"old-label": "value",
					},
				},
				Data: map[string]string{
					"old-key": "old-value",
				},
			},
			new: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
				Data: map[string]string{},
			},
			expected: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
				Data: map[string]string{},
			},
		},
		{
			name: "update with same values",
			old: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"label": "value",
					},
				},
				Data: map[string]string{
					"key": "value",
				},
			},
			new: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"label": "value",
					},
				},
				Data: map[string]string{
					"key": "value",
				},
			},
			expected: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"label": "value",
					},
				},
				Data: map[string]string{
					"key": "value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateConfigMap(tt.old, tt.new)
			if !equalConfigMaps(tt.old, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, tt.old)
			}
		})
	}
}

// Helper function to compare two Secrets
func equalSecrets(a, b *corev1.Secret) bool {
	if len(a.Labels) != len(b.Labels) || len(a.Data) != len(b.Data) || len(a.StringData) != len(b.StringData) {
		return false
	}

	for k, v := range a.Labels {
		if b.Labels[k] != v {
			return false
		}
	}

	for k, v := range a.Data {
		if !jsonEqual(b.Data[k], v) {
			return false
		}
	}

	for k, v := range a.StringData {
		if b.StringData[k] != v {
			return false
		}
	}

	return a.Type == b.Type
}

// TestUpdateSecret tests the UpdateSecret function.
func TestUpdateSecret(t *testing.T) {
	tests := []struct {
		name     string
		old      *corev1.Secret
		new      *corev1.Secret
		expected *corev1.Secret
	}{
		{
			name: "update with empty new Secret",
			old: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"old-label": "value",
					},
					Annotations: map[string]string{
						corev1.ServiceAccountNameKey: DefaultServiceAccountName,
					},
				},
				Data: map[string][]byte{
					"old-key": []byte("old-value"),
				},
				Type: corev1.SecretTypeDockerConfigJson,
			},
			new: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
				Data: map[string][]byte{},
				Type: corev1.SecretTypeBasicAuth,
			},
			expected: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
					Annotations: map[string]string{
						corev1.ServiceAccountNameKey: DefaultServiceAccountName,
					},
				},
				Data: map[string][]byte{},
				Type: corev1.SecretTypeOpaque,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateSecret(tt.old, tt.new)
			if !equalSecrets(tt.old, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, tt.old)
			}
		})
	}
}

// Helper function to compare two unstructured objects
func equalUnstructured(a, b *unstructured.Unstructured) bool {
	return a.GetName() == b.GetName() &&
		a.GetNamespace() == b.GetNamespace()
}

// TestUpdateUnstructured tests the updateUnstructured function
func TestUpdateUnstructured(t *testing.T) {
	tests := []struct {
		name     string
		old      *unstructured.Unstructured
		new      *unstructured.Unstructured
		oldObj   *corev1.ConfigMap
		newObj   *corev1.ConfigMap
		expected *unstructured.Unstructured
	}{
		{
			name: "update configmap labels",
			old: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "example-config",
						"namespace": "default",
						"labels": map[string]interface{}{
							"old-label": "value",
						},
					},
					"data": map[string]interface{}{
						"key": "old-value",
					},
				},
			},
			new: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"new-label": "new-value",
						},
					},
					"data": map[string]interface{}{
						"key": "new-value",
					},
				},
			},
			oldObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-config",
					Namespace: "default",
					Labels: map[string]string{
						"old-label": "value",
					},
				},
				Data: map[string]string{
					"key": "old-value",
				},
			},
			newObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"new-label": "new-value",
					},
				},
				Data: map[string]string{
					"key": "new-value",
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "example-config",
						"namespace": "default",
						"labels": map[string]interface{}{
							"new-label": "new-value",
						},
					},
					"data": map[string]interface{}{
						"key": "new-value",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UpdateUnstructured(tt.old, tt.new, tt.oldObj, tt.newObj, func(old, new *corev1.ConfigMap) {
				old.Labels = new.Labels
				old.Data = new.Data
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !equalUnstructured(result, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

// TestIsObjectGlobal tests the IsObjectGlobal function.
func TestIsObjectGlobal(t *testing.T) {
	tests := []struct {
		name     string
		obj      *metav1.ObjectMeta
		expected bool
	}{
		{
			name: "object without annotations",
			obj: &metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
			expected: false,
		},
		{
			name: "object with global annotation set to false",
			obj: &metav1.ObjectMeta{
				Annotations: map[string]string{
					KosmosGlobalLabel: "false",
				},
			},
			expected: false,
		},
		{
			name: "object with global annotation set to true",
			obj: &metav1.ObjectMeta{
				Annotations: map[string]string{
					KosmosGlobalLabel: "true",
				},
			},
			expected: true,
		},
		{
			name: "object with other annotations",
			obj: &metav1.ObjectMeta{
				Annotations: map[string]string{
					"other-annotation": "value",
					KosmosGlobalLabel:  "true",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsObjectGlobal(tt.obj)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsObjectUnstructuredGlobal tests the IsObjectUnstructuredGlobal function.
func TestIsObjectUnstructuredGlobal(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]string
		expected bool
	}{
		{
			name:     "nil object",
			obj:      nil,
			expected: false,
		},
		{
			name:     "empty object",
			obj:      map[string]string{},
			expected: false,
		},
		{
			name: "object with global label set to false",
			obj: map[string]string{
				KosmosGlobalLabel: "false",
			},
			expected: false,
		},
		{
			name: "object with global label set to true",
			obj: map[string]string{
				KosmosGlobalLabel: "true",
			},
			expected: true,
		},
		{
			name: "object with other keys",
			obj: map[string]string{
				"other-key":       "value",
				KosmosGlobalLabel: "true",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsObjectUnstructuredGlobal(tt.obj)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Helper function to compare two maps
func equalMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// TestAddResourceClusters tests the AddResourceClusters function
func TestAddResourceClusters(t *testing.T) {
	tests := []struct {
		name        string
		anno        map[string]string
		clusterName string
		expected    map[string]string
	}{
		{
			name:        "nil annotations",
			anno:        nil,
			clusterName: "cluster1",
			expected: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1",
			},
		},
		{
			name:        "empty annotations",
			anno:        map[string]string{},
			clusterName: "cluster1",
			expected: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1",
			},
		},
		{
			name: "existing cluster",
			anno: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,cluster2",
			},
			clusterName: "cluster1",
			expected: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,cluster2",
			},
		},
		{
			name: "adding new cluster",
			anno: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,cluster2",
			},
			clusterName: "cluster3",
			expected: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,cluster2,cluster3",
			},
		},
		{
			name: "adding empty cluster",
			anno: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,cluster2",
			},
			clusterName: "",
			expected: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,cluster2,",
			},
		},
		{
			name: "multiple empty clusters",
			anno: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,,cluster2,",
			},
			clusterName: "cluster3",
			expected: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,cluster2,cluster3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AddResourceClusters(tt.anno, tt.clusterName)
			if !equalMaps(result, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestHasResourceClusters(t *testing.T) {
	tests := []struct {
		name        string
		anno        map[string]string
		clusterName string
		expected    bool
	}{
		{
			name:        "nil annotations",
			anno:        nil,
			clusterName: "cluster1",
			expected:    false,
		},
		{
			name:        "empty annotations",
			anno:        map[string]string{},
			clusterName: "cluster1",
			expected:    false,
		},
		{
			name: "existing cluster",
			anno: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,cluster2",
			},
			clusterName: "cluster1",
			expected:    true,
		},
		{
			name: "non-existing cluster",
			anno: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,cluster2",
			},
			clusterName: "cluster3",
			expected:    false,
		},
		{
			name: "empty cluster name",
			anno: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,cluster2",
			},
			clusterName: "",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasResourceClusters(tt.anno, tt.clusterName)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Helper function to compare two string slices
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// TestListResourceClusters tests the ListResourceClusters function
func TestListResourceClusters(t *testing.T) {
	tests := []struct {
		name     string
		anno     map[string]string
		expected []string
	}{
		{
			name:     "nil annotations",
			anno:     nil,
			expected: []string{},
		},
		{
			name:     "empty annotations",
			anno:     map[string]string{},
			expected: []string{},
		},
		{
			name: "single cluster",
			anno: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1",
			},
			expected: []string{"cluster1"},
		},
		{
			name: "multiple clusters",
			anno: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,cluster2,cluster3",
			},
			expected: []string{"cluster1", "cluster2", "cluster3"},
		},
		{
			name: "clusters with empty values",
			anno: map[string]string{
				KosmosResourceOwnersAnnotations: "cluster1,,cluster3,",
			},
			expected: []string{"cluster1", "", "cluster3", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ListResourceClusters(tt.anno)
			if !equalStringSlices(result, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}
