package utils

import (
	"encoding/json"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/metrics/pkg/client/clientset/versioned"

	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

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

type Opts func(*rest.Config)

func NewConfigFromBytes(kubeConfig []byte, opts ...Opts) (*rest.Config, error) {
	var (
		config *rest.Config
		err    error
	)

	c, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
	if err != nil {
		return nil, err
	}
	config, err = c.ClientConfig()
	if err != nil {
		return nil, err
	}

	for _, h := range opts {
		if h == nil {
			continue
		}
		h(config)
	}

	return config, nil
}

func NewClientFromConfigPath(configPath string, opts ...Opts) (kubernetes.Interface, error) {
	var (
		config *rest.Config
		err    error
	)
	config, err = clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from configpath: %v", err)
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create clientset: %v", err)
	}
	return client, nil
}

func NewKosmosClientFromConfigPath(configPath string, opts ...Opts) (kosmosversioned.Interface, error) {
	var (
		config *rest.Config
		err    error
	)
	config, err = clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from configpath: %v", err)
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	client, err := kosmosversioned.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create clientset: %v", err)
	}
	return client, nil
}

func NewClientFromBytes(kubeConfig []byte, opts ...Opts) (kubernetes.Interface, error) {
	var (
		config *rest.Config
		err    error
	)

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
	if err != nil {
		return nil, err
	}
	config, err = clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create client failed: %v", err)
	}
	return client, nil
}

func NewKosmosClientFromBytes(kubeConfig []byte, opts ...Opts) (kosmosversioned.Interface, error) {
	var (
		config *rest.Config
		err    error
	)

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
	if err != nil {
		return nil, err
	}
	config, err = clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	client, err := kosmosversioned.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create client failed: %v", err)
	}
	return client, nil
}

func NewMetricClient(configPath string, opts ...Opts) (versioned.Interface, error) {
	var (
		config *rest.Config
		err    error
	)
	config, err = clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("could not read config file for cluster: %v", err)
		}
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	metricClient, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create client for root cluster: %v", err)
	}
	return metricClient, nil
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
	old.Type = new.Type
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
