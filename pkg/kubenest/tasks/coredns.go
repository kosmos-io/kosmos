package tasks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewCoreDNSTask() workflow.Task {
	return workflow.Task{
		Name:        "apiserver",
		Run:         runApiserver,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-core-dns-in-host-cluster",
				Run:  runCoreDnsHostTask,
			},
			{
				Name: "check-core-dns",
				Run:  runCheckCoreDnsTask,
			},
			{
				Name: "deploy-core-dns-service-in-virtual-cluster",
				Run:  runCoreDnsVirtualTask,
			},
		},
	}
}

func getCoreDnsHostComponentsConfig(client clientset.Interface, keyName string) ([]ComponentConfig, error) {
	cm, err := client.CoreV1().ConfigMaps(constants.KosmosNs).Get(context.Background(), constants.ManifestComponentsConfigmap, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	yamlData, ok := cm.Data[keyName]
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

func ApplyYMLTemplate(dynamicClient dynamic.Interface, manifestGlob string, templateMapping map[string]interface{}) error {
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

		templateBytes, err := util.ParseTemplate(string(bytesData), templateMapping)
		if err != nil {
			return errors.Wrapf(err, "Parse template %s error", manifest)
		}

		err = yaml.Unmarshal(templateBytes, &obj)
		if err != nil {
			return errors.Wrapf(err, "Unmarshal manifest bytes data error")
		}

		err = createObject(dynamicClient, obj.GetNamespace(), obj.GetName(), &obj)
		if err != nil {
			return errors.Wrapf(err, "Create object error")
		}
	}
	return nil
}

// in host
func runCoreDnsHostTask(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster manifests-components task invoked with an invalid data struct")
	}

	dynamicClient := data.DynamicClient()

	components, err := getCoreDnsHostComponentsConfig(data.RemoteClient(), constants.HostCoreDnsComponents)
	if err != nil {
		return err
	}

	imageRepository, _ := util.GetImageMessage()

	for _, component := range components {
		klog.V(2).Infof("Deploy component %s", component.Name)

		templatedMapping := map[string]interface{}{
			"Namespace":       data.GetNamespace(),
			"Name":            data.GetName(),
			"ImageRepository": imageRepository,
		}
		err = ApplyYMLTemplate(dynamicClient, component.Path, templatedMapping)
		if err != nil {
			return err
		}
	}
	return nil
}

// in host
func runCheckCoreDnsTask(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster manifests-components task invoked with an invalid data struct")
	}
	ctx := context.TODO()

	waitCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	isReady := false

	wait.UntilWithContext(waitCtx, func(ctx context.Context) {
		_, err := data.RemoteClient().CoreV1().Services(data.GetNamespace()).Get(context.TODO(), constants.KubeDNSSVCName, metav1.GetOptions{})
		if err == nil {
			// TODO: check endpoints
			isReady = true
			cancel()
		}
	}, 10*time.Second) // Interval time

	if isReady {
		return nil
	}

	return fmt.Errorf("kube-dns is not ready")
}

func runCoreDnsVirtualTask(r workflow.RunData) error {
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

	components, err := getCoreDnsHostComponentsConfig(data.RemoteClient(), constants.VirtualCoreDnsComponents)
	if err != nil {
		return err
	}

	kubesvc, err := data.RemoteClient().CoreV1().Services(data.GetNamespace()).Get(context.TODO(), constants.KubeDNSSVCName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	DNSPort := int32(0)
	DNSTCPPort := int32(0)
	MetricsPort := int32(0)

	for _, port := range kubesvc.Spec.Ports {
		if port.Name == "dns" {
			DNSPort = port.NodePort
		}
		if port.Name == "dns-tcp" {
			DNSTCPPort = port.NodePort
		}
		if port.Name == "metrics" {
			MetricsPort = port.NodePort
		}
	}
	HostNodeAddress := os.Getenv("EXECTOR_HOST_MASTER_NODE_IP")
	if len(HostNodeAddress) == 0 {
		return fmt.Errorf("get master node ip from env failed")
	}

	for _, component := range components {
		klog.V(2).Infof("Deploy component %s", component.Name)

		templatedMapping := map[string]interface{}{
			"Namespace":       data.GetNamespace(),
			"Name":            data.GetName(),
			"DNSPort":         DNSPort,
			"DNSTCPPort":      DNSTCPPort,
			"MetricsPort":     MetricsPort,
			"HostNodeAddress": HostNodeAddress,
		}
		err = ApplyYMLTemplate(dynamicClient, component.Path, templatedMapping)
		if err != nil {
			return err
		}
	}
	return nil
}
