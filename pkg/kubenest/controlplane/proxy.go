package controlplane

import (
	"fmt"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/proxy"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureVirtualClusterProxy(client clientset.Interface, kubeconfigString, clusterCIDR string) error {
	// install kube-proxy ds in virtual cluster
	if err := installProxyDaemonSet(client); err != nil {
		return fmt.Errorf("failed to install virtual cluster proxy, err: %w", err)
	}

	// install kube-proxy cm in virtual cluster
	if err := installProxyConfigMap(client, kubeconfigString, clusterCIDR); err != nil {
		return fmt.Errorf("failed to install virtual cluster proxy, err: %w", err)
	}

	// install kube-proxy sa in virtual cluster
	if err := installProxySA(client); err != nil {
		return fmt.Errorf("failed to install virtual cluster proxy, err: %w", err)
	}
	return nil
}

func DeleteVirtualClusterProxy(client clientset.Interface) error {
	daemonSetName := fmt.Sprintf("%s-%s", "kube", "proxy")
	daemonSetNameSpace := fmt.Sprintf("%s-%s", "kube", "system")
	if err := util.DeleteDaemonSet(client, daemonSetName, daemonSetNameSpace); err != nil {
		return errors.Wrapf(err, "Failed to delete daemonSet %s/%s", daemonSetName, daemonSetNameSpace)
	}

	cmName := fmt.Sprintf("%s-%s", "kube", "proxy")
	cmNameSpace := fmt.Sprintf("%s-%s", "kube", "system")
	if err := util.DeleteConfigmap(client, cmName, cmNameSpace); err != nil {
		return errors.Wrapf(err, "Failed to delete ConfigMap %s/%s", cmName, cmNameSpace)
	}

	saName := fmt.Sprintf("%s-%s", "kube", "proxy")
	saNameSpace := fmt.Sprintf("%s-%s", "kube", "system")
	if err := util.DeleteServiceAccount(client, saName, saNameSpace); err != nil {
		return errors.Wrapf(err, "Failed to delete ServiceAccount %s/%s", saName, saNameSpace)
	}
	return nil
}

func installProxyDaemonSet(client clientset.Interface) error {
	imageRepository, imageVersion := util.GetImageMessage()

	proxyDaemonSetBytes, err := util.ParseTemplate(proxy.ProxyDaemonSet, struct {
		DaemonSetName, Namespace, ImageRepository, Version string
	}{
		DaemonSetName:   fmt.Sprintf("%s-%s", "kube", "proxy"),
		Namespace:       fmt.Sprintf("%s-%s", "kube", "system"),
		ImageRepository: imageRepository,
		Version:         imageVersion,
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtual cluster proxy daemonSet template: %w", err)
	}

	proxyDaemonSet := &appsv1.DaemonSet{}
	if err := yaml.Unmarshal([]byte(proxyDaemonSetBytes), proxyDaemonSet); err != nil {
		return fmt.Errorf("error when decoding virtual cluster proxy daemonSet: %w", err)
	}

	if err := util.CreateOrUpdateDaemonSet(client, proxyDaemonSet); err != nil {
		return fmt.Errorf("error when creating daemonSet for %s, err: %w", proxyDaemonSet.Name, err)
	}
	return nil
}

func installProxyConfigMap(client clientset.Interface, kubeconfigString, clusterCIDR string) error {
	proxyConfigMapBytes, err := util.ParseTemplate(proxy.ProxyConfigMap, struct {
		ConfigMapName, Namespace, KubeProxyKubeConfig, ClusterCIDR string
	}{
		ConfigMapName:       fmt.Sprintf("%s-%s", "kube", "proxy"),
		Namespace:           fmt.Sprintf("%s-%s", "kube", "system"),
		KubeProxyKubeConfig: kubeconfigString,
		ClusterCIDR:         clusterCIDR,
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtual cluster proxy configmap template: %w", err)
	}

	proxyConfigMap := &corev1.ConfigMap{}
	if err := yaml.Unmarshal([]byte(proxyConfigMapBytes), proxyConfigMap); err != nil {
		return fmt.Errorf("error when decoding virtual cluster proxy configmap: %w", err)
	}

	if err := util.CreateOrUpdateConfigMap(client, proxyConfigMap); err != nil {
		return fmt.Errorf("error when creating configmap for %s, err: %w", proxyConfigMap.Name, err)
	}
	return nil
}

func installProxySA(client clientset.Interface) error {
	proxySABytes, err := util.ParseTemplate(proxy.ProxySA, struct {
		SAName, Namespace string
	}{
		SAName:    fmt.Sprintf("%s-%s", "kube", "proxy"),
		Namespace: fmt.Sprintf("%s-%s", "kube", "system"),
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtual cluster proxy SA template: %w", err)
	}

	proxySA := &corev1.ServiceAccount{}
	if err := yaml.Unmarshal([]byte(proxySABytes), proxySA); err != nil {
		return fmt.Errorf("error when decoding virtual cluster proxy SA: %w", err)
	}

	if err := util.CreateOrUpdateServiceAccount(client, proxySA); err != nil {
		return fmt.Errorf("error when creating SA for %s, err: %w", proxySA.Name, err)
	}
	return nil
}
