package apiserver

import (
	"context"
	"errors"
	"fmt"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/apiserver"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

var errAllocated = errors.New("provided port is already allocated")

func EnsureVirtualClusterAPIServer(client clientset.Interface, name, namespace string) error {
	if err := installAPIServer(client, name, namespace); err != nil {
		return fmt.Errorf("failed to install virtual cluster apiserver, err: %w", err)
	}
	return nil
}

func installAPIServer(client clientset.Interface, name, namespace string) error {
	imageRepository, imageVersion := util.GetImageMessage()
	err, clusterIps := util.GetEtcdServiceClusterIp(namespace, client)
	if err != nil {
		return nil
	}

	apiserverDeploymentBytes, err := util.ParseTemplate(apiserver.ApiserverDeployment, struct {
		DeploymentName, Namespace, ImageRepository, EtcdClientService, Version string
		ServiceSubnet, VirtualClusterCertsSecret, EtcdCertsSecret              string
		Replicas                                                               int32
		EtcdListenClientPort                                                   int32
	}{
		DeploymentName:            fmt.Sprintf("%s-%s", name, "apiserver"),
		Namespace:                 namespace,
		ImageRepository:           imageRepository,
		Version:                   imageVersion,
		EtcdClientService:         clusterIps[1],
		ServiceSubnet:             constants.ApiServerServiceSubnet,
		VirtualClusterCertsSecret: fmt.Sprintf("%s-%s", name, "cert"),
		EtcdCertsSecret:           fmt.Sprintf("%s-%s", name, "etcd-cert"),
		Replicas:                  constants.ApiServerReplicas,
		EtcdListenClientPort:      constants.ApiServerEtcdListenClientPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtual cluster apiserver deployment template: %w", err)
	}

	apiserverDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), apiserverDeploymentBytes, apiserverDeployment); err != nil {
		return fmt.Errorf("error when decoding virtual cluster apiserver deployment: %w", err)
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
