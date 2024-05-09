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

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	vcnodecontroller "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/apiserver"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/etcd"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureVirtualClusterService(client clientset.Interface, name, namespace string, manager *vcnodecontroller.HostPortManager) error {
	port, err := manager.AllocateHostPort(name)
	if err != nil {
		return fmt.Errorf("failed to allocate host ip for virtual cluster apiserver, err: %w", err)
	}

	if err := createServerService(client, name, namespace, port); err != nil {
		return fmt.Errorf("failed to create virtual cluster apiserver-service, err: %w", err)
	}
	return nil
}

func DeleteVirtualClusterService(client clientset.Interface, name, namespace string, manager *vcnodecontroller.HostPortManager) error {
	services := []string{
		fmt.Sprintf("%s-%s", name, "apiserver"),
		fmt.Sprintf("%s-%s", name, "etcd"),
		fmt.Sprintf("%s-%s", name, "etcd-client"),
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

	err := manager.ReleaseHostPort(name)
	if err != nil {
		klog.Warningf("Error releasing host port for cluster %s: %v", name, err)
	}

	klog.V(2).Infof("Successfully uninstalled service for virtualcluster %s", name)
	return nil
}

func createServerService(client clientset.Interface, name, namespace string, port int32) error {
	apiserverServiceBytes, err := util.ParseTemplate(apiserver.ApiserverService, struct {
		ServiceName, Namespace, ServiceType string
		ServicePort                         int32
	}{
		ServiceName: fmt.Sprintf("%s-%s", name, "apiserver"),
		Namespace:   namespace,
		ServiceType: constants.ApiServerServiceType,
		ServicePort: port,
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtualClusterApiserver serive template: %w", err)
	}

	apiserverService := &corev1.Service{}
	if err := yaml.Unmarshal([]byte(apiserverServiceBytes), apiserverService); err != nil {
		return fmt.Errorf("error when decoding virtual cluster apiserver service: %w", err)
	}
	if err := createOrUpdateService(client, apiserverService); err != nil {
		return fmt.Errorf("err when creating virtual cluster apiserver service for %s, err: %w", apiserverService.Name, err)
	}

	etcdServicePeerBytes, err := util.ParseTemplate(etcd.EtcdPeerService, struct {
		ServiceName, Namespace                   string
		EtcdListenClientPort, EtcdListenPeerPort int32
	}{
		ServiceName:          fmt.Sprintf("%s-%s", name, "etcd"),
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

	if err := createOrUpdateService(client, etcdPeerService); err != nil {
		return fmt.Errorf("error when creating etcd client service, err: %w", err)
	}

	etcdClientServiceBytes, err := util.ParseTemplate(etcd.EtcdClientService, struct {
		ServiceName, Namespace string
		EtcdListenClientPort   int32
	}{
		ServiceName:          fmt.Sprintf("%s-%s", name, "etcd-client"),
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

	if err := createOrUpdateService(client, etcdClientService); err != nil {
		return fmt.Errorf("err when creating etcd client service, err: %w", err)
	}

	return nil
}

func createOrUpdateService(client clientset.Interface, service *corev1.Service) error {
	_, err := client.CoreV1().Services(service.GetNamespace()).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "Failed to create service %s/%s", service.GetName(), service.GetNamespace())
		}

		older, err := client.CoreV1().Services(service.GetNamespace()).Get(context.TODO(), service.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		service.ResourceVersion = older.ResourceVersion
		if _, err := client.CoreV1().Services(service.GetNamespace()).Update(context.TODO(), service, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update Service: %v", err)
		}
	}

	klog.V(5).InfoS("Successfully created or updated service", "service", service.GetName())
	return nil
}
