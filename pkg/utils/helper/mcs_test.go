// nolint:dupl
package helper

import (
	"testing"

	disv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

func TestAddEndpointSliceAnnotation(t *testing.T) {
	t.Run("Annotations is not nil", func(t *testing.T) {
		eps := &disv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"testKey": "testValue",
				},
			},
		}

		AddEndpointSliceAnnotation(eps, "newKey", "newValue")

		if eps.Annotations["newKey"] != "newValue" {
			t.Errorf("Expected 'newValue', got '%s'", eps.Annotations["newKey"])
		}
	})

	t.Run("Annotations is nil", func(t *testing.T) {
		eps := &disv1.EndpointSlice{}

		AddEndpointSliceAnnotation(eps, "newKey", "newValue")

		if eps.Annotations["newKey"] != "newValue" {
			t.Errorf("Expected 'newValue', got '%s'", eps.Annotations["newKey"])
		}
	})
}

func TestAddServiceImportAnnotation(t *testing.T) {
	t.Run("Annotations is not nil", func(t *testing.T) {
		serviceImport := &v1alpha1.ServiceImport{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"testKey": "testValue",
				},
			},
		}

		AddServiceImportAnnotation(serviceImport, "newKey", "newValue")

		if serviceImport.Annotations["newKey"] != "newValue" {
			t.Errorf("Expected 'newValue', got '%s'", serviceImport.Annotations["newKey"])
		}
	})

	t.Run("Annotations is nil", func(t *testing.T) {
		serviceImport := &v1alpha1.ServiceImport{}

		AddServiceImportAnnotation(serviceImport, "newKey", "newValue")

		if serviceImport.Annotations["newKey"] != "newValue" {
			t.Errorf("Expected 'newValue', got '%s'", serviceImport.Annotations["newKey"])
		}
	})
}

func TestAddEndpointSliceLabel(t *testing.T) {
	t.Run("Labels is not nil", func(t *testing.T) {
		eps := &disv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"testKey": "testValue",
				},
			},
		}

		AddEndpointSliceLabel(eps, "newKey", "newValue")

		if eps.Labels["newKey"] != "newValue" {
			t.Errorf("Expected 'newValue', got '%s'", eps.Labels["newKey"])
		}
	})

	t.Run("Labels is nil", func(t *testing.T) {
		eps := &disv1.EndpointSlice{}

		AddEndpointSliceLabel(eps, "newKey", "newValue")

		if eps.Labels["newKey"] != "newValue" {
			t.Errorf("Expected 'newValue', got '%s'", eps.Labels["newKey"])
		}
	})
}

func TestGetLabelOrAnnotationValue(t *testing.T) {
	t.Run("Value exists", func(t *testing.T) {
		values := map[string]string{
			"testKey": "testValue",
		}

		value := GetLabelOrAnnotationValue(values, "testKey")

		if value != "testValue" {
			t.Errorf("Expected 'testValue', got '%s'", value)
		}
	})

	t.Run("Value does not exist", func(t *testing.T) {
		values := map[string]string{
			"testKey": "testValue",
		}

		value := GetLabelOrAnnotationValue(values, "nonExistentKey")

		if value != "" {
			t.Errorf("Expected '', got '%s'", value)
		}
	})

	t.Run("Values is nil", func(t *testing.T) {
		var values map[string]string

		value := GetLabelOrAnnotationValue(values, "testKey")
		if value != "" {
			t.Errorf("Expected '', got '%s'", value)
		}
	})
}

func TestRemoveAnnotation(t *testing.T) {
	t.Run("Annotation exists", func(t *testing.T) {
		eps := &disv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"testKey": "testValue",
				},
			},
		}

		RemoveAnnotation(eps, "testKey")

		if _, exists := eps.GetAnnotations()["testKey"]; exists {
			t.Errorf("Expected annotation 'testKey' to be removed, but it still exists")
		}
	})

	t.Run("Annotation does not exist", func(t *testing.T) {
		eps := &disv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"testKey": "testValue",
				},
			},
		}

		RemoveAnnotation(eps, "nonExistentKey")

		if _, exists := eps.GetAnnotations()["nonExistentKey"]; exists {
			t.Errorf("Expected annotation 'nonExistentKey' to not exist, but it does")
		}
	})

	t.Run("Annotations is nil", func(t *testing.T) {
		eps := &disv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{},
		}

		RemoveAnnotation(eps, "testKey")

		if _, exists := eps.GetAnnotations()["testKey"]; exists {
			t.Errorf("Expected annotation 'testKey' to not exist, but it does")
		}
	})

	t.Run("Remove annotation from nil annotations", func(t *testing.T) {
		eps := &disv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{},
		}

		RemoveAnnotation(eps, "testKey")

		if len(eps.GetAnnotations()) != 0 {
			t.Errorf("Expected annotations to be empty, but got %v", eps.GetAnnotations())
		}
	})

	t.Run("Remove non-existing annotation", func(t *testing.T) {
		eps := &disv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"existingKey": "existingValue",
				},
			},
		}

		RemoveAnnotation(eps, "nonExistingKey")

		if len(eps.GetAnnotations()) != 1 {
			t.Errorf("Expected annotations to contain 1 key, but got %v", eps.GetAnnotations())
		}
	})
}

func TestHasAnnotation(t *testing.T) {
	t.Run("Annotation exists", func(t *testing.T) {
		m := metav1.ObjectMeta{
			Annotations: map[string]string{
				"testKey": "testValue",
			},
		}

		if !HasAnnotation(m, "testKey") {
			t.Errorf("Expected annotation 'testKey' to exist, but it does not")
		}
	})

	t.Run("Annotation does not exist", func(t *testing.T) {
		m := metav1.ObjectMeta{
			Annotations: map[string]string{
				"testKey": "testValue",
			},
		}

		if HasAnnotation(m, "nonExistentKey") {
			t.Errorf("Expected annotation 'nonExistentKey' to not exist, but it does")
		}
	})

	t.Run("Annotations is nil", func(t *testing.T) {
		m := metav1.ObjectMeta{}

		if HasAnnotation(m, "testKey") {
			t.Errorf("Expected annotation 'testKey' to not exist, but it does")
		}
	})
}

func TestGetAnnotationValue(t *testing.T) {
	t.Run("Annotation exists", func(t *testing.T) {
		m := metav1.ObjectMeta{
			Annotations: map[string]string{
				"testKey": "testValue",
			},
		}

		value, found := GetAnnotationValue(m, "testKey")

		if !found {
			t.Errorf("Expected annotation 'testKey' to exist, but it does not")
		}
		if value != "testValue" {
			t.Errorf("Expected 'testValue', got '%v'", value)
		}
	})

	t.Run("Annotation does not exist", func(t *testing.T) {
		m := metav1.ObjectMeta{
			Annotations: map[string]string{
				"testKey": "testValue",
			},
		}

		value, found := GetAnnotationValue(m, "nonExistentKey")

		if found {
			t.Errorf("Expected annotation 'nonExistentKey' to not exist, but it does")
		}
		if value != "" {
			t.Errorf("Expected empty string, got '%v'", value)
		}
	})

	t.Run("Annotations is nil", func(t *testing.T) {
		m := metav1.ObjectMeta{}
		value, found := GetAnnotationValue(m, "testKey")

		if found {
			t.Errorf("Expected annotation 'testKey' to not exist, but it does")
		}
		if value != "" {
			t.Errorf("Expected empty string, got '%v'", value)
		}
	})
}
