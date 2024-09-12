package controlplane

import (
	"fmt"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/apiserver"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureVirtualClusterAPIServer(client clientset.Interface, name, namespace string, portMap map[string]int32, kubeNestConfiguration *v1alpha1.KubeNestConfiguration, vc *v1alpha1.VirtualCluster) error {
	if err := installAPIServer(client, name, namespace, portMap, kubeNestConfiguration, vc); err != nil {
		return fmt.Errorf("failed to install virtual cluster apiserver, err: %w", err)
	}
	return nil
}

func DeleteVirtualClusterAPIServer(client clientset.Interface, name, namespace string) error {
	deployName := util.GetAPIServerName(name)
	if err := util.DeleteDeployment(client, deployName, namespace); err != nil {
		return errors.Wrapf(err, "Failed to delete deployment %s/%s", deployName, namespace)
	}
	return nil
}

func installAPIServer(client clientset.Interface, name, namespace string, portMap map[string]int32, kubeNestConfiguration *v1alpha1.KubeNestConfiguration, vc *v1alpha1.VirtualCluster) error {
	imageRepository, imageVersion := util.GetImageMessage()
	clusterIP, err := util.GetEtcdServiceClusterIP(namespace, name+constants.EtcdSuffix, client)
	if err != nil {
		return nil
	}

	vclabel := util.GetVirtualControllerLabel()

	IPV6FirstFlag, err := util.IPV6First(constants.APIServerServiceSubnet)
	if err != nil {
		return err
	}

	apiserverDeploymentBytes, err := util.ParseTemplate(apiserver.ApiserverDeployment, struct {
		DeploymentName, Namespace, ImageRepository, EtcdClientService, Version, VirtualControllerLabel string
		ServiceSubnet, VirtualClusterCertsSecret, EtcdCertsSecret                                      string
		Replicas                                                                                       int
		EtcdListenClientPort                                                                           int32
		ClusterPort                                                                                    int32
		AdmissionPlugins                                                                               bool
		IPV6First                                                                                      bool
		UseAPIServerNodePort                                                                           bool
	}{
		DeploymentName:            util.GetAPIServerName(name),
		Namespace:                 namespace,
		ImageRepository:           imageRepository,
		Version:                   imageVersion,
		VirtualControllerLabel:    vclabel,
		EtcdClientService:         clusterIP,
		ServiceSubnet:             constants.APIServerServiceSubnet,
		VirtualClusterCertsSecret: util.GetCertName(name),
		EtcdCertsSecret:           util.GetEtcdCertName(name),
		Replicas:                  kubeNestConfiguration.KubeInKubeConfig.APIServerReplicas,
		EtcdListenClientPort:      constants.APIServerEtcdListenClientPort,
		ClusterPort:               portMap[constants.APIServerPortKey],
		IPV6First:                 IPV6FirstFlag,
		AdmissionPlugins:          kubeNestConfiguration.KubeInKubeConfig.AdmissionPlugins,
		UseAPIServerNodePort:      vc.Spec.KubeInKubeConfig != nil && vc.Spec.KubeInKubeConfig.APIServerServiceType == v1alpha1.NodePort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtual cluster apiserver deployment template: %w", err)
	}

	apiserverDeployment := &appsv1.Deployment{}
	if err := yaml.Unmarshal([]byte(apiserverDeploymentBytes), apiserverDeployment); err != nil {
		return fmt.Errorf("error when decoding virtual cluster apiserver deployment: %w", err)
	}

	if err := util.CreateOrUpdateDeployment(client, apiserverDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", apiserverDeployment.Name, err)
	}
	return nil
}
