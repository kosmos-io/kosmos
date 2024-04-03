package apiserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/treeoperator/constants"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

var errAllocated = errors.New("provided port is already allocated")

const (
	Replicas             = 1
	ImageDefaultVersion  = "v1.26.12"
	ServiceSubnet        = "10.237.6.18/29"
	EtcdListenClientPort = 2379
	Type                 = "NodePort"
)

func EnsureVirtualClusterAPIServer(client clientset.Interface, name, namespace string) error {
	if err := installAPIServer(client, name, namespace); err != nil {
		return fmt.Errorf("failed to install virtual cluster apiserver, err: %w", err)
	}
	//createAPIServerService(client, name, namespace)
	return createAPIServerService(client, name, namespace)
}

func installAPIServer(client clientset.Interface, name, namespace string) error {
	imageRepository := os.Getenv(constants.DefauleImageRepositoryEnv)
	if len(imageRepository) == 0 {
		imageRepository = utils.DefaultImageRepository
	}
	apiserverDeploymentBytes, err := util.ParseTemplate(ApiserverDeployment, struct {
		DeploymentName, Namespace, ImageRepository, EtcdClientService, Version string
		ServiceSubnet, VirtualClusterCertsSecret, EtcdCertsSecret              string
		Replicas                                                               int32
		EtcdListenClientPort                                                   int32
	}{
		DeploymentName:            fmt.Sprintf("%s-%s", name, "apiserver"),
		Namespace:                 namespace,
		ImageRepository:           imageRepository,
		Version:                   ImageDefaultVersion,
		EtcdClientService:         fmt.Sprintf("%s-%s", name, "etcd-client"),
		ServiceSubnet:             ServiceSubnet,
		VirtualClusterCertsSecret: fmt.Sprintf("%s-%s", name, "cert"),
		EtcdCertsSecret:           fmt.Sprintf("%s-%s", name, "etcd-cert"),
		Replicas:                  Replicas,
		EtcdListenClientPort:      EtcdListenClientPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmadaApiserver deployment template: %w", err)
	}

	apiserverDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), apiserverDeploymentBytes, apiserverDeployment); err != nil {
		return fmt.Errorf("error when decoding karmadaApiserver deployment: %w", err)
	}

	if err := createOrUpdateDeployment(client, apiserverDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", apiserverDeployment.Name, err)
	}
	return nil
}

func createOrUpdateDeployment(client clientset.Interface, deployment *appsv1.Deployment) error {
	_, err := client.AppsV1().Deployments(deployment.GetNamespace()).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		_, err := client.AppsV1().Deployments(deployment.GetNamespace()).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated deployment", "deployment", deployment.GetName())
	return nil
}

func createAPIServerService(client clientset.Interface, name, namespace string) error {
	apiserverServiceBytes, err := util.ParseTemplate(ApiserverService, struct {
		ServiceName, Namespace, ServiceType string
	}{
		ServiceName: fmt.Sprintf("%s-%s", name, "apiserver"),
		Namespace:   namespace,
		ServiceType: Type,
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtualClusterApiserver serive template: %w", err)
	}

	apiserverService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), apiserverServiceBytes, apiserverService); err != nil {
		return fmt.Errorf("error when decoding virtualClusterApiserver serive: %w", err)
	}

	if err := createOrUpdateService(client, apiserverService); err != nil {
		return fmt.Errorf("err when creating service for %s, err: %w", apiserverService.Name, err)
	}
	return nil
}

func createOrUpdateService(client clientset.Interface, service *corev1.Service) error {
	_, err := client.CoreV1().Services(service.GetNamespace()).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			// Ignore if the Service is invalid with this error message:
			// Service "apiserver" is invalid: provided Port is already allocated.
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
