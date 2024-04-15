package controlplane

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/apiserver"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/etcd"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureVirtualClusterService(client clientset.Interface, name, namespace string) error {
	if err := createServerService(client, name, namespace); err != nil {
		return fmt.Errorf("failed to create virtual cluster apiserver-service, err: %w", err)
	}
	return nil
}

func createServerService(client clientset.Interface, name, namespace string) error {
	apiserverServiceBytes, err := util.ParseTemplate(apiserver.ApiserverService, struct {
		ServiceName, Namespace, ServiceType string
	}{
		ServiceName: fmt.Sprintf("%s-%s", name, "apiserver"),
		Namespace:   namespace,
		ServiceType: constants.ApiServerServiceType,
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtualClusterApiserver serive template: %w", err)
	}

	apiserverService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), apiserverServiceBytes, apiserverService); err != nil {
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
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), etcdServicePeerBytes, etcdPeerService); err != nil {
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
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), etcdClientServiceBytes, etcdClientService); err != nil {
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
			if apierrors.IsInvalid(err) && strings.Contains(err.Error(), errAllocated.Error()) {
				klog.V(2).ErrorS(err, "failed to create or update service", "service", klog.KObj(service))
				return nil
			}
			return fmt.Errorf("unable to create Service: %v", err)
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
