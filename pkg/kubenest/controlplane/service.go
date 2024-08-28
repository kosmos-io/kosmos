package controlplane

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/apiserver"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/coredns/host"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/etcd"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

func EnsureVirtualClusterService(client clientset.Interface, name, namespace string, portMap map[string]int32, kubeNestOpt *v1alpha1.KubeNestConfiguration, vc *v1alpha1.VirtualCluster) error {
	if err := createServerService(client, name, namespace, portMap, kubeNestOpt, vc); err != nil {
		return fmt.Errorf("failed to create virtual cluster apiserver-service, err: %w", err)
	}
	return nil
}

func DeleteVirtualClusterService(client clientset.Interface, name, namespace string) error {
	services := []string{
		util.GetApiServerName(name),
		util.GetEtcdServerName(name),
		util.GetEtcdClientServerName(name),
		"kube-dns",
		util.GetKonnectivityServerName(name),
		util.GetKonnectivityApiServerName(name),
	}
	for _, service := range services {
		err := client.CoreV1().Services(namespace).Delete(context.TODO(), service, metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.V(2).Infof("Service %s/%s not found, skip delete", service, namespace)
				continue
			}
			return errors.Wrapf(err, "Failed to delete service %s/%s", service, namespace)
		}
	}

	klog.V(2).Infof("Successfully uninstalled service for virtualcluster %s", name)
	return nil
}

func createServerService(client clientset.Interface, name, namespace string, portMap map[string]int32, _ *v1alpha1.KubeNestConfiguration, vc *v1alpha1.VirtualCluster) error {
	ipFamilies := utils.IPFamilyGenerator(constants.ApiServerServiceSubnet)
	apiserverServiceBytes, err := util.ParseTemplate(apiserver.ApiserverService, struct {
		ServiceName, Namespace, ServiceType string
		ServicePort                         int32
		IPFamilies                          []corev1.IPFamily
		UseApiServerNodePort                bool
	}{
		ServiceName:          util.GetApiServerName(name),
		Namespace:            namespace,
		ServiceType:          constants.ApiServerServiceType,
		ServicePort:          portMap[constants.ApiServerPortKey],
		IPFamilies:           ipFamilies,
		UseApiServerNodePort: vc.Spec.KubeInKubeConfig != nil && vc.Spec.KubeInKubeConfig.ApiServerServiceType == v1alpha1.NodePort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtualClusterApiserver serive template: %w", err)
	}
	anpServiceBytes, err := util.ParseTemplate(apiserver.ApiserverAnpService, struct {
		ServiceName, Namespace string
		ProxyServerPort        int32
	}{
		ServiceName:     util.GetKonnectivityServerName(name),
		Namespace:       namespace,
		ProxyServerPort: portMap[constants.ApiServerNetworkProxyServerPortKey],
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtualClusterApiserver anp service template: %w", err)
	}

	apiserverService := &corev1.Service{}
	if err := yaml.Unmarshal([]byte(apiserverServiceBytes), apiserverService); err != nil {
		return fmt.Errorf("error when decoding virtual cluster apiserver service: %w", err)
	}
	if err := util.CreateOrUpdateService(client, apiserverService); err != nil {
		return fmt.Errorf("err when creating virtual cluster apiserver service for %s, err: %w", apiserverService.Name, err)
	}

	anpService := &corev1.Service{}
	if err := yaml.Unmarshal([]byte(anpServiceBytes), anpService); err != nil {
		return fmt.Errorf("error when decoding virtual cluster anp service: %w", err)
	}
	if err := util.CreateOrUpdateService(client, anpService); err != nil {
		return fmt.Errorf("err when creating virtual cluster anp service for %s, err: %w", anpService.Name, err)
	}

	etcdServicePeerBytes, err := util.ParseTemplate(etcd.EtcdPeerService, struct {
		ServiceName, Namespace                   string
		EtcdListenClientPort, EtcdListenPeerPort int32
	}{
		ServiceName:          util.GetEtcdServerName(name),
		Namespace:            namespace,
		EtcdListenClientPort: constants.EtcdListenClientPort,
		EtcdListenPeerPort:   constants.EtcdListenPeerPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing Etcd client serive template: %w", err)
	}

	etcdPeerService := &corev1.Service{}
	if err := yaml.Unmarshal([]byte(etcdServicePeerBytes), etcdPeerService); err != nil {
		return fmt.Errorf("error when decoding Etcd client service: %w", err)
	}

	if err := util.CreateOrUpdateService(client, etcdPeerService); err != nil {
		return fmt.Errorf("error when creating etcd client service, err: %w", err)
	}

	//etcd-client service
	etcdClientServiceBytes, err := util.ParseTemplate(etcd.EtcdClientService, struct {
		ServiceName, Namespace string
		EtcdListenClientPort   int32
	}{
		ServiceName:          util.GetEtcdClientServerName(name),
		Namespace:            namespace,
		EtcdListenClientPort: constants.EtcdListenClientPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing Etcd client serive template: %w", err)
	}

	etcdClientService := &corev1.Service{}
	if err := yaml.Unmarshal([]byte(etcdClientServiceBytes), etcdClientService); err != nil {
		return fmt.Errorf("err when decoding Etcd client service: %w", err)
	}

	if err := util.CreateOrUpdateService(client, etcdClientService); err != nil {
		return fmt.Errorf("err when creating etcd client service, err: %w", err)
	}

	//core-dns service
	coreDnsServiceBytes, err := util.ParseTemplate(host.CoreDnsService, struct {
		Namespace string
	}{
		Namespace: namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing core-dns serive template: %w", err)
	}

	coreDnsService := &corev1.Service{}
	if err := yaml.Unmarshal([]byte(coreDnsServiceBytes), coreDnsService); err != nil {
		return fmt.Errorf("err when decoding core-dns service: %w", err)
	}

	if err := util.CreateOrUpdateService(client, coreDnsService); err != nil {
		return fmt.Errorf("err when creating core-dns service, err: %w", err)
	}

	return nil
}
