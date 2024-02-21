package utils

import (
	"encoding/json"
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

// IsKosmosNode judge whether node is kosmos's node
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

func AddResourceClusters(anno map[string]string, clusterName string) map[string]string {
	if anno == nil {
		anno = map[string]string{}
	}
	owners := strings.Split(anno[KosmosResourceOwnersAnnotations], ",")
	newowners := make([]string, 0)

	flag := false
	for _, v := range owners {
		if len(v) == 0 {
			continue
		}
		newowners = append(newowners, v)
		if v == clusterName {
			// already existed
			flag = true
		}
	}

	if !flag {
		newowners = append(newowners, clusterName)
	}

	anno[KosmosResourceOwnersAnnotations] = strings.Join(newowners, ",")
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

// IsNotReady judge whether node is not ready
func IsNotReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return false
		}
	}
	return true
}
