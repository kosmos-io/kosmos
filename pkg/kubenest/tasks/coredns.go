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
	apiclient "github.com/kosmos.io/kosmos/pkg/kubenest/util/api-client"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewCoreDNSTask() workflow.Task {
	return workflow.Task{
		Name:        "coreDns",
		Run:         runCoreDNS,
		Skip:        skipCoreDNS,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-core-dns-in-host-cluster",
				Run:  runCoreDNSHostTask,
			},
			{
				Name: "check-core-dns",
				Run:  runCheckCoreDNSTask,
			},
			{
				Name: "deploy-core-dns-service-in-virtual-cluster",
				Run:  runCoreDNSVirtualTask,
			},
		},
	}
}

func skipCoreDNS(d workflow.RunData) (bool, error) {
	data, ok := d.(InitData)
	if !ok {
		return false, errors.New("coreDns task invoked with an invalid data struct")
	}

	vc := data.VirtualCluster()
	if vc.Spec.KubeInKubeConfig != nil && vc.Spec.KubeInKubeConfig.UseTenantDNS {
		return true, nil
	}
	return false, nil
}

func runCoreDNS(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("coreDns task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[coreDns] Running coreDns task", "virtual cluster", klog.KObj(data))
	return nil
}

func UninstallCoreDNSTask() workflow.Task {
	return workflow.Task{
		Name:        "coredns",
		Run:         runCoreDNS,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "remove-core-dns-in-host-cluster",
				Run:  uninstallCorednsHostTask,
			},
		},
	}
}

func getCoreDNSHostComponentsConfig(client clientset.Interface, keyName string) ([]ComponentConfig, error) {
	cm, err := client.CoreV1().ConfigMaps(constants.KosmosNs).Get(context.Background(), constants.ManifestComponentsConfigMap, metav1.GetOptions{})
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

// in host
func runCoreDNSHostTask(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster manifests-components task invoked with an invalid data struct")
	}

	dynamicClient := data.DynamicClient()

	components, err := getCoreDNSHostComponentsConfig(data.RemoteClient(), constants.HostCoreDnsComponents)
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
		for k, v := range data.PluginOptions() {
			templatedMapping[k] = v
		}
		err = applyYMLTemplate(dynamicClient, component.Path, templatedMapping)
		if err != nil {
			return err
		}
	}
	return nil
}

func uninstallCorednsHostTask(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster manifests-components task invoked with an invalid data struct")
	}

	dynamicClient := data.DynamicClient()

	components, err := getCoreDNSHostComponentsConfig(data.RemoteClient(), constants.HostCoreDnsComponents)
	if err != nil {
		return err
	}

	imageRepository, _ := util.GetImageMessage()

	for _, component := range components {
		klog.V(2).Infof("Delete component %s", component.Name)

		templatedMapping := map[string]interface{}{
			"Namespace":       data.GetNamespace(),
			"Name":            data.GetName(),
			"ImageRepository": imageRepository,
		}
		err = deleteYMLTemplate(dynamicClient, component.Path, templatedMapping)
		if err != nil {
			return err
		}
	}
	return nil
}

// in host
func runCheckCoreDNSTask(r workflow.RunData) error {
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

func runCoreDNSVirtualTask(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster coreDns task invoked with an invalid data struct")
	}

	secret, err := data.RemoteClient().CoreV1().Secrets(data.GetNamespace()).Get(context.TODO(),
		util.GetAdminConfigSecretName(data.GetName()), metav1.GetOptions{})
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

	components, err := getCoreDNSHostComponentsConfig(data.RemoteClient(), constants.VirtualCoreDNSComponents)
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
		for k, v := range data.PluginOptions() {
			templatedMapping[k] = v
		}
		err = applyYMLTemplate(dynamicClient, component.Path, templatedMapping)
		if err != nil {
			return err
		}
	}
	return nil
}

// nolint:dupl
func applyYMLTemplate(dynamicClient dynamic.Interface, manifestGlob string, templateMapping map[string]interface{}) error {
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

		err = yaml.Unmarshal([]byte(templateBytes), &obj)
		if err != nil {
			return errors.Wrapf(err, "Unmarshal manifest bytes data error")
		}

		err = apiclient.TryRunCommand(func() error {
			return util.ApplyObject(dynamicClient, &obj)
		}, 3)
		if err != nil {
			return errors.Wrapf(err, "Create object error")
		}
	}
	return nil
}

// nolint:dupl
func deleteYMLTemplate(dynamicClient dynamic.Interface, manifestGlob string, templateMapping map[string]interface{}) error {
	manifests, err := filepath.Glob(manifestGlob)
	klog.V(2).Infof("Component Manifests %s", manifestGlob)
	if err != nil {
		return err
	}
	if manifests == nil {
		return errors.Errorf("No matching file for pattern %v", manifestGlob)
	}
	for _, manifest := range manifests {
		klog.V(2).Infof("Deleting %s", manifest)
		var obj unstructured.Unstructured
		bytesData, err := os.ReadFile(manifest)
		if err != nil {
			return errors.Wrapf(err, "Read file %s error", manifest)
		}

		templateBytes, err := util.ParseTemplate(string(bytesData), templateMapping)
		if err != nil {
			return errors.Wrapf(err, "Parse template %s error", manifest)
		}

		err = yaml.Unmarshal([]byte(templateBytes), &obj)
		if err != nil {
			return errors.Wrapf(err, "Unmarshal manifest bytes data error")
		}

		err = util.DeleteObject(dynamicClient, obj.GetNamespace(), obj.GetName(), &obj)
		if err != nil {
			return errors.Wrapf(err, "Delete object error")
		}
	}
	return nil
}
