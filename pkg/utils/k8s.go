package utils

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// Todo rename the filename

type ClustersNodeSelection struct {
	NodeSelector              map[string]string                 `json:"nodeSelector,omitempty"`
	Affinity                  *corev1.Affinity                  `json:"affinity,omitempty"`
	Tolerations               []corev1.Toleration               `json:"tolerations,omitempty"`
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

type EnvResourceManager interface {
	GetConfigMap(name, namespace string) (*corev1.ConfigMap, error)
	GetSecret(name, namespace string) (*corev1.Secret, error)
	ListServices() ([]*corev1.Service, error)
}

func CreateMergePatch(original, new interface{}) ([]byte, error) {
	originBytes, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}
	cloneBytes, err := json.Marshal(new)
	if err != nil {
		return nil, err
	}
	patch, err := jsonpatch.CreateMergePatch(originBytes, cloneBytes)
	if err != nil {
		return nil, err
	}
	return patch, nil
}

func IsKosmosNode(node *corev1.Node) bool {
	if node == nil {
		return false
	}
	labelVal, exist := node.ObjectMeta.Labels[KosmosNodeLabel]
	if !exist {
		return false
	}
	return labelVal == KosmosNodeValue
}

func IsExcludeNode(node *corev1.Node) bool {
	if node == nil {
		return false
	}
	labelVal, exist := node.ObjectMeta.Labels[KosmosExcludeNodeLabel]
	if !exist {
		return false
	}
	return labelVal == KosmosExcludeNodeValue
}

func IsVirtualPod(pod *corev1.Pod) bool {
	if pod.Labels != nil && pod.Labels[KosmosPodLabel] == "true" {
		return true
	}
	return false
}

func UpdateConfigMap(old, new *corev1.ConfigMap) {
	old.Labels = new.Labels
	old.Data = new.Data
	old.BinaryData = new.BinaryData
}

func UpdateSecret(old, new *corev1.Secret) {
	old.Labels = new.Labels
	old.Data = new.Data
	old.StringData = new.StringData
	// The satoken type of default in a subset group is Opaque
	if old.Annotations[corev1.ServiceAccountNameKey] == DefaultServiceAccountName {
		old.Type = corev1.SecretTypeOpaque
	} else {
		old.Type = new.Type
	}
}

func UpdateUnstructured[T *corev1.ConfigMap | *corev1.Secret](old, new *unstructured.Unstructured, oldObj T, newObj T, update func(old, new T)) (*unstructured.Unstructured, error) {
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(old.UnstructuredContent(), &oldObj); err != nil {
		return nil, err
	}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(new.UnstructuredContent(), &newObj); err != nil {
		return nil, err
	}

	update(oldObj, newObj)

	if retObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(oldObj); err == nil {
		return &unstructured.Unstructured{
			Object: retObj,
		}, nil
	} else {
		return nil, err
	}
}

func IsObjectGlobal(obj *metav1.ObjectMeta) bool {
	if obj.Annotations == nil {
		return false
	}

	if obj.Annotations[KosmosGlobalLabel] == "true" {
		return true
	}

	return false
}

func IsObjectUnstructuredGlobal(obj map[string]string) bool {
	if obj == nil {
		return false
	}

	if obj[KosmosGlobalLabel] == "true" {
		return true
	}

	return false
}

func ResetMetadata(obj interface{}) error {
	value := reflect.ValueOf(obj)
	if value.Kind() != reflect.Ptr {
		return fmt.Errorf("obj must be a pointer")
	}

	value = value.Elem()
	metaField := value.FieldByName("ObjectMeta")
	if !metaField.IsValid() || metaField.Kind() != reflect.Struct {
		return fmt.Errorf("obj does not have an ObjectMeta field")
	}
	metaField.FieldByName("ResourceVersion").SetString("")
	metaField.FieldByName("UID").SetString("")
	metaField.FieldByName("Generation").SetInt(0)
	metaField.FieldByName("SelfLink").SetString("")
	ownerRefsField := metaField.FieldByName("OwnerReferences")
	if ownerRefsField.IsValid() && ownerRefsField.Kind() == reflect.Slice && ownerRefsField.CanSet() {
		ownerRefsField.Set(reflect.MakeSlice(ownerRefsField.Type(), 0, 0))
	}
	return nil
}

func IsImmediateModePvc(annotations map[string]string) bool {
	if _, ok := annotations[KosmosPvcImmediateMode]; ok {
		return true
	}
	return false
}

func AddResourceClusters(anno map[string]string, clusterName string) map[string]string {
	if anno == nil {
		anno = make(map[string]string)
	}

	ownerStr := anno[KosmosResourceOwnersAnnotations]
	if ownerStr == "" {
		anno[KosmosResourceOwnersAnnotations] = clusterName
		return anno
	}

	owners := strings.Split(ownerStr, ",")
	found := false
	for _, v := range owners {
		if strings.TrimSpace(v) == clusterName {
			found = true
			break
		}
	}

	if !found {
		owners = append(owners, clusterName)
		anno[KosmosResourceOwnersAnnotations] = strings.Join(owners, ",")
	}
	return anno
}

func HasResourceClusters(anno map[string]string, clusterName string) bool {
	if anno == nil {
		anno = map[string]string{}
	}
	owners := strings.Split(anno[KosmosResourceOwnersAnnotations], ",")

	for _, v := range owners {
		if v == clusterName {
			// already existed
			return true
		}
	}
	return false
}

func ListResourceClusters(anno map[string]string) []string {
	if anno == nil || anno[KosmosResourceOwnersAnnotations] == "" {
		return []string{}
	}
	owners := strings.Split(anno[KosmosResourceOwnersAnnotations], ",")
	return owners
}
