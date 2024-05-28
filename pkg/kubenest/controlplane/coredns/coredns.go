package coredns

import (
	"fmt"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/coredns/host"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/coredns/virtualcluster"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureHostCoreDns(client clientset.Interface, name, namespace string) error {
	err := installCoreDnsConfigMap(client, namespace)
	if err != nil {
		return err
	}

	err = EnsureCoreDnsRBAC(client, namespace, name)
	if err != nil {
		return err
	}

	err = installCoreDnsDeployment(client, name, namespace)
	if err != nil {
		return err
	}
	return nil
}

func EnsureVirtualClusterCoreDns(dynamicClient dynamic.Interface, templateMapping map[string]interface{}) error {
	err := installCoreDnsEndpointsInVirtualCluster(dynamicClient, templateMapping)
	if err != nil {
		return err
	}

	err = installCoreDnsServiceInVirtualCluster(dynamicClient, templateMapping)
	if err != nil {
		return err
	}
	return nil
}

func installCoreDnsDeployment(client clientset.Interface, name, namespace string) error {
	imageRepository, _ := util.GetImageMessage()
	imageTag := util.GetCoreDnsImageTag()
	coreDnsDeploymentBytes, err := util.ParseTemplate(host.CoreDnsDeployment, struct {
		Namespace, Name, ImageRepository, CoreDNSImageTag string
	}{
		Namespace:       namespace,
		Name:            name,
		ImageRepository: imageRepository,
		CoreDNSImageTag: imageTag,
	})
	if err != nil {
		return fmt.Errorf("error when parsing core-dns deployment template: %w", err)
	}
	coreDnsDeployment := &appsv1.Deployment{}
	if err := yaml.Unmarshal([]byte(coreDnsDeploymentBytes), coreDnsDeployment); err != nil {
		return fmt.Errorf("error when decoding core-dns deployment: %w", err)
	}

	if err := util.CreateOrUpdateDeployment(client, coreDnsDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", coreDnsDeployment.Name, err)
	}
	return nil
}

func getCoreDnsConfigMapManifest(namespace string) (*v1.ConfigMap, error) {
	coreDnsConfigMapBytes, err := util.ParseTemplate(host.CoreDnsCM, struct {
		Namespace string
	}{
		Namespace: namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("error when parsing core-dns configMap template: %w", err)
	}

	config := &v1.ConfigMap{}
	if err := yaml.Unmarshal([]byte(coreDnsConfigMapBytes), config); err != nil {
		return nil, fmt.Errorf("err when decoding core-dns configMap: %w", err)
	}

	return config, nil
}

func installCoreDnsConfigMap(client clientset.Interface, namespace string) error {
	config, err := getCoreDnsConfigMapManifest(namespace)
	if err != nil {
		return err
	}

	if err := util.CreateOrUpdateConfigMap(client, config); err != nil {
		return fmt.Errorf("error when creating configMap for %s, err: %w", config.Name, err)
	}
	return nil
}

func installCoreDnsServiceInVirtualCluster(dynamicClient dynamic.Interface, templateMapping map[string]interface{}) error {
	coreDnsServiceInVcBytes, err := util.ParseTemplate(virtualcluster.CoreDnsService, templateMapping)
	if err != nil {
		return fmt.Errorf("error when parsing core-dns service in virtual cluster template: %w", err)
	}
	var obj unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(coreDnsServiceInVcBytes), &obj); err != nil {
		return fmt.Errorf("err when decoding core-dns service in virtual cluster: %w", err)
	}

	err = util.CreateObject(dynamicClient, obj.GetNamespace(), obj.GetName(), &obj)
	if err != nil {
		return fmt.Errorf("error when creating core-dns service in virtual cluster err: %w", err)
	}
	return nil
}

func installCoreDnsEndpointsInVirtualCluster(dynamicClient dynamic.Interface, templateMapping map[string]interface{}) error {
	coreDnsEndpointsInVcBytes, err := util.ParseTemplate(virtualcluster.CoreDnsEndpoints, templateMapping)
	if err != nil {
		return fmt.Errorf("error when parsing core-dns service in virtual cluster template: %w", err)
	}
	var obj unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(coreDnsEndpointsInVcBytes), &obj); err != nil {
		return fmt.Errorf("err when decoding core-dns service in virtual cluster: %w", err)
	}

	err = util.CreateObject(dynamicClient, obj.GetNamespace(), obj.GetName(), &obj)
	if err != nil {
		return fmt.Errorf("error when creating core-dns service in virtual cluster err: %w", err)
	}
	return nil
}

func DeleteCoreDnsDeployment(client clientset.Interface, name, namespace string) error {
	// delete deployment
	deployName := fmt.Sprintf("%s-%s", name, "coredns")
	if err := util.DeleteDeployment(client, deployName, namespace); err != nil {
		return errors.Wrapf(err, "Failed to delete deployment %s/%s", deployName, namespace)
	}

	// delete configmap
	cmName := "coredns"
	if err := util.DeleteConfigmap(client, cmName, namespace); err != nil {
		return errors.Wrapf(err, "Failed to delete configmap %s/%s", cmName, namespace)
	}

	return nil
}
