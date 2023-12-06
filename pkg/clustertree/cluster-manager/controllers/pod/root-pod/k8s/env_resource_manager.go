package rootpodsyncers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

type envResourceManager struct {
	DynamicRootClient dynamic.Interface
}

// GetConfigMap retrieves the specified config map from the cache.
func (rm *envResourceManager) GetConfigMap(name, namespace string) (*corev1.ConfigMap, error) {
	// return rm.configMapLister.ConfigMaps(namespace).Get(name)
	obj, err := rm.DynamicRootClient.Resource(utils.GVR_CONFIGMAP).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	retObj := &corev1.ConfigMap{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &retObj); err != nil {
		return nil, err
	}

	return retObj, nil
}

// GetSecret retrieves the specified secret from Kubernetes.
func (rm *envResourceManager) GetSecret(name, namespace string) (*corev1.Secret, error) {
	// return rm.secretLister.Secrets(namespace).Get(name)
	obj, err := rm.DynamicRootClient.Resource(utils.GVR_SECRET).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	retObj := &corev1.Secret{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &retObj); err != nil {
		return nil, err
	}

	return retObj, nil
}

// ListServices retrieves the list of services from Kubernetes.
func (rm *envResourceManager) ListServices() ([]*corev1.Service, error) {
	// return rm.serviceLister.List(labels.Everything())
	objs, err := rm.DynamicRootClient.Resource(utils.GVR_SERVICE).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
	})

	if err != nil {
		return nil, err
	}

	retObj := make([]*corev1.Service, 0)

	for _, obj := range objs.Items {
		tmpObj := &corev1.Service{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &tmpObj); err != nil {
			return nil, err
		}
		retObj = append(retObj, tmpObj)
	}

	return retObj, nil
}

func NewEnvResourceManager(client dynamic.Interface) utils.EnvResourceManager {
	return &envResourceManager{
		DynamicRootClient: client,
	}
}
