package controlplane

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	controller "github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/kube-controller"
	scheduler "github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/scheduler"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureControlPlaneComponent(component, name, namespace string, client clientset.Interface) error {
	configMaps, err := getComponentConfigMapManifests(name, namespace)
	if err != nil {
		return err
	}
	configMap, ok := configMaps[constants.VirtualClusterSchedulerComponentConfigMap]
	if !ok {
		klog.Infof("Skip installing component configMap  %s(%s/%s)", component, namespace, name)
		return nil
	}

	if err := util.CreateOrUpdateConfigMap(client, configMap); err != nil {
		return fmt.Errorf("failed to create configMap resource for component %s, err: %w", component, err)
	}

	deployments, err := getComponentManifests(name, namespace)
	if err != nil {
		return err
	}

	deployment, ok := deployments[component]
	if !ok {
		klog.Infof("Skip installing component %s(%s/%s)", component, namespace, name)
		return nil
	}

	if err := util.CreateOrUpdateDeployment(client, deployment); err != nil {
		return fmt.Errorf("failed to create deployment resource for component %s, err: %w", component, err)
	}

	return nil
}

func getComponentManifests(name, namespace string) (map[string]*appsv1.Deployment, error) {
	kubeControllerManager, err := getKubeControllerManagerManifest(name, namespace)
	if err != nil {
		return nil, err
	}
	virtualClusterScheduler, err := getVirtualClusterSchedulerManifest(name, namespace)
	if err != nil {
		return nil, err
	}

	manifest := map[string]*appsv1.Deployment{
		constants.KubeControllerManagerComponent:   kubeControllerManager,
		constants.VirtualClusterSchedulerComponent: virtualClusterScheduler,
	}

	return manifest, nil
}

func getComponentConfigMapManifests(name, namespace string) (map[string]*v1.ConfigMap, error) {
	virtualClusterSchedulerConfigMap, err := getVirtualClusterSchedulerConfigMapManifest(name, namespace)
	if err != nil {
		return nil, err
	}

	manifest := map[string]*v1.ConfigMap{
		constants.VirtualClusterSchedulerComponentConfigMap: virtualClusterSchedulerConfigMap,
	}

	return manifest, nil
}

func getKubeControllerManagerManifest(name, namespace string) (*appsv1.Deployment, error) {
	imageRepository, imageVersion := util.GetImageMessage()
	kubeControllerManagerBytes, err := util.ParseTemplate(controller.KubeControllerManagerDeployment, struct {
		DeploymentName, Namespace, ImageRepository, Version string
		VirtualClusterCertsSecret, KubeconfigSecret         string
		Replicas                                            int32
	}{
		DeploymentName:            fmt.Sprintf("%s-%s", name, "kube-controller-manager"),
		Namespace:                 namespace,
		ImageRepository:           imageRepository,
		Version:                   imageVersion,
		VirtualClusterCertsSecret: fmt.Sprintf("%s-%s", name, "cert"),
		KubeconfigSecret:          fmt.Sprintf("%s-%s", name, "admin-config"),
		Replicas:                  constants.KubeControllerReplicas,
	})
	if err != nil {
		return nil, fmt.Errorf("error when parsing kube-controller-manager deployment template: %w", err)
	}

	kcm := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), kubeControllerManagerBytes, kcm); err != nil {
		return nil, fmt.Errorf("err when decoding kube-controller-manager deployment: %w", err)
	}

	return kcm, nil
}

func getVirtualClusterSchedulerConfigMapManifest(name, namespace string) (*v1.ConfigMap, error) {
	virtualClusterSchedulerConfigMapBytes, err := util.ParseTemplate(scheduler.VirtualClusterSchedulerConfigMap, struct {
		DeploymentName, Namespace string
	}{
		DeploymentName: fmt.Sprintf("%s-%s", name, "virtualcluster-scheduler"),
		Namespace:      namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("error when parsing virtualCluster-scheduler configMap template: %w", err)
	}

	scheduler := &v1.ConfigMap{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), virtualClusterSchedulerConfigMapBytes, scheduler); err != nil {
		return nil, fmt.Errorf("err when decoding virtualCluster-scheduler configMap: %w", err)
	}

	return scheduler, nil
}

func getVirtualClusterSchedulerManifest(name, namespace string) (*appsv1.Deployment, error) {
	imageRepository, imageVersion := util.GetImageMessage()
	virtualClusterSchedulerBytes, err := util.ParseTemplate(scheduler.VirtualClusterSchedulerDeployment, struct {
		Replicas                                                             int32
		DeploymentName, Namespace, SystemNamespace, ImageRepository, Version string
		Image, KubeconfigSecret                                              string
	}{
		DeploymentName:   fmt.Sprintf("%s-%s", name, "virtualcluster-scheduler"),
		Namespace:        namespace,
		SystemNamespace:  constants.SystemNs,
		ImageRepository:  imageRepository,
		Version:          imageVersion,
		KubeconfigSecret: fmt.Sprintf("%s-%s", name, "admin-config"),
		Replicas:         constants.VirtualClusterSchedulerReplicas,
	})
	if err != nil {
		return nil, fmt.Errorf("error when parsing virtualCluster-scheduler deployment template: %w", err)
	}

	scheduler := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), virtualClusterSchedulerBytes, scheduler); err != nil {
		return nil, fmt.Errorf("err when decoding virtualCluster-scheduler deployment: %w", err)
	}

	return scheduler, nil
}
