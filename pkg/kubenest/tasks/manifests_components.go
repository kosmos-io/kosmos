package tasks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

type ComponentConfig struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path" yaml:"path"`
}

func NewComponentsFromManifestsTask() workflow.Task {
	return workflow.Task{
		Name:        "manifests-components",
		Run:         runComponentsFromManifests,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-manifests-components",
				Run:  applyComponentsManifests,
			},
		},
	}
}

func runComponentsFromManifests(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("manifests-components task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[apiserver] Running manifests-components task", "virtual cluster", klog.KObj(data))
	return nil
}

func applyComponentsManifests(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster manifests-components task invoked with an invalid data struct")
	}

	secret, err := data.RemoteClient().CoreV1().Secrets(data.GetNamespace()).Get(context.TODO(),
		fmt.Sprintf("%s-%s", data.GetName(), constants.AdminConfig), metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Get virtualcluster kubeconfig secret error")
	}
	config, err := clientcmd.RESTConfigFromKubeConfig(secret.Data[constants.KubeConfig])
	if err != nil {
		return err
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	components, err := getComponentsConfig(data.RemoteClient())
	if err != nil {
		return err
	}

	for _, component := range components {
		klog.V(2).Infof("Deploy component %s", component.Name)

		templatedMapping := make(map[string]interface{}, 2)
		if component.Name == constants.VirtualClusterKubeProxyComponent {
			templatedMapping["KUBE_PROXY_KUBECONFIG"] = string(secret.Data[constants.KubeConfig])
		}
		err = applyTemplatedManifests(dynamicClient, component.Path, templatedMapping)
		if err != nil {
			return err
		}
	}

	klog.V(2).InfoS("[manifests-components] Successfully installed virtual cluster manifests-components", "virtual cluster", klog.KObj(data))
	return nil
}

func getComponentsConfig(client clientset.Interface) ([]ComponentConfig, error) {
	cm, err := client.CoreV1().ConfigMaps(constants.KosmosNs).Get(context.Background(), constants.ManifestComponentsConfigmap, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	yamlData, ok := cm.Data["components"]
	if !ok {
		return nil, errors.Wrap(err, "Read manifests components config error")
	}

	var components []ComponentConfig
	err = yaml.Unmarshal([]byte(yamlData), &components)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshal manifests component config error")
	}
	return components, nil
}

func applyTemplatedManifests(dynamicClient dynamic.Interface, manifestGlob string, templateMapping map[string]interface{}) error {
	manifests, err := filepath.Glob(manifestGlob)
	klog.V(2).Infof("Component Manifests %s", manifestGlob)
	if err != nil {
		return err
	}
	if manifests == nil {
		return errors.Errorf("No matching file for pattern %v", manifestGlob)
	}
	for _, manifest := range manifests {
		klog.V(2).Infof("Applying %s", manifest)
		var obj unstructured.Unstructured
		bytesData, err := os.ReadFile(manifest)
		if err != nil {
			return errors.Wrapf(err, "Read file %s error", manifest)
		}
		err = yaml.Unmarshal(bytesData, &obj)
		if err != nil {
			return errors.Wrapf(err, "Unmarshal manifest bytes data error")
		}
		gvk := obj.GroupVersionKind()
		gvr, _ := meta.UnsafeGuessKindToResource(gvk)
		if obj.GetName() == constants.KubeProxyConfigmap && gvr.Resource == "configmaps" {
			cm := &corev1.ConfigMap{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, cm)
			if err != nil {
				return errors.Wrapf(err, "Convert unstructured obj to configmap %s error", obj.GetName())
			}
			cm.Data["kubeconfig.conf"] = templateMapping["KUBE_PROXY_KUBECONFIG"].(string)
			res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
			if err != nil {
				return errors.Wrapf(err, "Convert configmap %s to unstructured obj error", obj.GetName())
			}
			obj = unstructured.Unstructured{Object: res}
		} else {
			templatedBytes, err := util.ParseTemplate(string(bytesData), templateMapping)
			if err != nil {
				return errors.Wrapf(err, "Parse template data %s", string(bytesData))
			}
			err = yaml.Unmarshal(templatedBytes, &obj)
			if err != nil {
				return errors.Wrapf(err, "Unmarshal templatedBytes error")
			}
		}

		err = createObject(dynamicClient, obj.GetNamespace(), obj.GetName(), &obj)
		if err != nil {
			return errors.Wrapf(err, "Create object error")
		}
	}
	return nil
}

func createObject(dynamicClient dynamic.Interface, namespace string, name string, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	klog.V(2).Infof("Create %s, name: %s, namespace: %s", gvr.String(), name, namespace)
	_, err := dynamicClient.Resource(gvr).Namespace(namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Warningf("%s %s already exists", gvr.String(), name)
			return nil
		} else {
			return err
		}
	}
	return nil
}
