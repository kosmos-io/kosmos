package controlplane

import (
	"fmt"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	vcnodecontroller "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/apiserver"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureVirtualClusterAPIServer(client clientset.Interface, name, namespace string, manager *vcnodecontroller.HostPortManager) error {
	_, err := manager.AllocateHostIP(name)
	if err != nil {
		return fmt.Errorf("failed to allocate host ip for virtual cluster apiserver, err: %w", err)
	}

	if err := installAPIServer(client, name, namespace); err != nil {
		return fmt.Errorf("failed to install virtual cluster apiserver, err: %w", err)
	}
	return nil
}

func DeleteVirtualClusterAPIServer(client clientset.Interface, name, namespace string) error {
	deployName := fmt.Sprintf("%s-%s", name, "apiserver")
	if err := util.DeleteDeployment(client, deployName, namespace); err != nil {
		return errors.Wrapf(err, "Failed to delete deployment %s/%s", deployName, namespace)
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

	if err := util.CreateOrUpdateDeployment(client, apiserverDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", apiserverDeployment.Name, err)
	}
	return nil
}
