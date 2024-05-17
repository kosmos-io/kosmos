package tasks

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/apiserver"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewAnpTask() workflow.Task {
	return workflow.Task{
		Name:        "anp",
		Run:         runAnp,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-anp-server",
				Run:  runAnpServer,
			},
			{
				Name: "deploy-anp-agent",
				Run:  runAnpAgent,
			},
		},
	}
}

func runAnp(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("anp task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[anp] Running anp task", "virtual cluster", klog.KObj(data))
	return nil
}

func runAnpServer(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster anp task invoked with an invalid data struct")
	}
	// install egress_selector_configuration config map
	egressSelectorConfig, err := util.ParseTemplate(apiserver.EgressSelectorConfiguration, struct {
		Namespace string
	}{
		Namespace: data.GetNamespace(),
	})
	if err != nil {
		return fmt.Errorf("failed to parse egress_selector_configuration config map template, err: %w", err)
	}
	cm := &v1.ConfigMap{}
	err = yaml.Unmarshal([]byte(egressSelectorConfig), cm)
	if err != nil {
		return fmt.Errorf("failed to parse egress_selector_configuration config map template, err: %w", err)
	}
	// create configMap
	err = util.CreateOrUpdateConfigMap(data.RemoteClient(), cm)
	if err != nil {
		return fmt.Errorf("failed to create egress_selector_configuration config map, err: %w", err)
	}
	err = installAnpServer(data.RemoteClient(), data.GetName(), data.GetNamespace(), data.HostPortMap())
	if err != nil {
		return fmt.Errorf("failed to install virtual cluster anp component, err: %w", err)
	}

	klog.V(2).InfoS("[VirtualClusterAnp] Successfully installed virtual cluster anp component", "virtual cluster", klog.KObj(data))
	return nil
}

func runAnpAgent(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("check-VirtualClusterAnp task invoked with an invalid data struct")
	}
	return installAnpAgent(data.RemoteClient(), data.GetName(), data.GetNamespace(), data.HostPortMap())
}

func UninstallAnpTask() workflow.Task {
	return workflow.Task{
		Name:        "anp",
		Run:         runAnp,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "anp",
				Run:  uninstallAnp,
			},
		},
	}
}

func uninstallAnp(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster anp task invoked with an invalid data struct")
	}

	anpManifest, vcClient, err := getAnpAgentManifest(data.RemoteClient(), data.GetName(), data.GetNamespace(), data.HostPortMap())
	if err != nil {
		return fmt.Errorf("failed to uninstall anp agent when get anp manifest, err: %w", err)
	}
	actionFunc := func(ctx context.Context, c dynamic.Interface, u *unstructured.Unstructured) error {
		// create the object
		return util.DeleteObject(vcClient, u.GetNamespace(), u.GetName(), u)
	}

	klog.V(2).InfoS("[VirtualClusterAnp] Successfully uninstalled virtual cluster anp component", "virtual cluster", klog.KObj(data))
	return util.ForEachObjectInYAML(context.TODO(), vcClient, []byte(anpManifest), "", actionFunc)
}
func installAnpServer(client clientset.Interface, name, namespace string, portMap map[string]int32) error {
	imageRepository, imageVersion := util.GetImageMessage()
	clusterIp, err := util.GetEtcdServiceClusterIp(namespace, name+constants.EtcdSuffix, client)
	if err != nil {
		return nil
	}

	apiserverDeploymentBytes, err := util.ParseTemplate(apiserver.ApiserverAnpDeployment, struct {
		DeploymentName, Namespace, ImageRepository, EtcdClientService, Version string
		ServiceSubnet, VirtualClusterCertsSecret, EtcdCertsSecret              string
		Replicas                                                               int32
		EtcdListenClientPort                                                   int32
		ClusterPort                                                            int32
		AgentPort                                                              int32
		ServerPort                                                             int32
		HealthPort                                                             int32
		AdminPort                                                              int32
		KubeconfigSecret                                                       string
		Name                                                                   string
	}{
		DeploymentName:            fmt.Sprintf("%s-%s", name, "apiserver"),
		Namespace:                 namespace,
		ImageRepository:           imageRepository,
		Version:                   imageVersion,
		EtcdClientService:         clusterIp,
		ServiceSubnet:             constants.ApiServerServiceSubnet,
		VirtualClusterCertsSecret: fmt.Sprintf("%s-%s", name, "cert"),
		EtcdCertsSecret:           fmt.Sprintf("%s-%s", name, "etcd-cert"),
		Replicas:                  constants.ApiServerReplicas,
		EtcdListenClientPort:      constants.ApiServerEtcdListenClientPort,
		ClusterPort:               portMap[constants.ApiServerPortKey],
		AgentPort:                 portMap[constants.ApiServerNetworkProxyAgentPortKey],
		ServerPort:                portMap[constants.ApiServerNetworkProxyServerPortKey],
		HealthPort:                portMap[constants.ApiServerNetworkProxyHealthPortKey],
		AdminPort:                 portMap[constants.ApiServerNetworkProxyAdminPortKey],
		KubeconfigSecret:          fmt.Sprintf("%s-%s", name, "admin-config-clusterip"),
		Name:                      name,
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtual cluster apiserver deployment template: %w", err)
	}
	klog.V(4).InfoS("[anp] apply anp server", "anp sever deploy", apiserverDeploymentBytes)

	apiserverDeployment := &appsv1.Deployment{}
	if err := yaml.Unmarshal([]byte(apiserverDeploymentBytes), apiserverDeployment); err != nil {
		return fmt.Errorf("error when decoding virtual cluster apiserver deployment: %w", err)
	}

	if err := util.CreateOrUpdateDeployment(client, apiserverDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", apiserverDeployment.Name, err)
	}
	return nil
}

func installAnpAgent(client clientset.Interface, name, namespace string, portMap map[string]int32) error {
	anpAgentManifestBytes, vcClient, err2 := getAnpAgentManifest(client, name, namespace, portMap)
	if err2 != nil {
		return err2
	}
	actionFunc := func(ctx context.Context, c dynamic.Interface, u *unstructured.Unstructured) error {
		// create the object
		return util.ApplyObject(vcClient, u)
	}
	return util.ForEachObjectInYAML(context.TODO(), vcClient, []byte(anpAgentManifestBytes), "", actionFunc)
}

func getAnpAgentManifest(client clientset.Interface, name string, namespace string, portMap map[string]int32) (string, dynamic.Interface, error) {
	imageRepository, imageVersion := util.GetImageMessage()
	// get apiServer hostIp
	proxyServerHost, err := getDeploymentPodIPs(client, namespace, fmt.Sprintf("%s-%s", name, "apiserver"))
	if err != nil {
		return "", nil, fmt.Errorf("error when get apiserver hostIp, err: %w", err)
	}

	anpAgentManifeattBytes, err := util.ParseTemplate(apiserver.AnpAgentManifest, struct {
		ImageRepository string
		Version         string
		AgentPort       int32
		ProxyServerHost []string
	}{
		ImageRepository: imageRepository,
		Version:         imageVersion,
		AgentPort:       portMap[constants.ApiServerNetworkProxyAgentPortKey],
		ProxyServerHost: proxyServerHost,
	})
	if err != nil {
		return "", nil, fmt.Errorf("error when parsing virtual cluster apiserver deployment template: %w", err)
	}
	klog.V(4).InfoS("[anp] apply anp agent", "agent manifest", anpAgentManifeattBytes)
	vcClient, err := getVcClient(client, name, namespace)
	if err != nil {
		return "", nil, fmt.Errorf("error when get vcClient, err: %v", err)
	}
	return anpAgentManifeattBytes, vcClient, nil
}

// getDeploymentPodIPs 获取指定 Deployment 的所有 Pod IP 地址
func getDeploymentPodIPs(clientset clientset.Interface, namespace, deploymentName string) ([]string, error) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting deployment: %v", err)
	}

	labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}

	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), listOptions)
	if err != nil {
		return nil, fmt.Errorf("error listing pods: %v", err)
	}

	var podIPs []string
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			podIPs = append(podIPs, pod.Status.PodIP)
		}
	}

	return podIPs, nil
}

func getVcClient(client clientset.Interface, name, namespace string) (dynamic.Interface, error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(),
		fmt.Sprintf("%s-%s", name, constants.AdminConfig), metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Get virtualcluster kubeconfig secret error")
	}
	config, err := clientcmd.RESTConfigFromKubeConfig(secret.Data[constants.KubeConfig])
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return dynamicClient, nil
}
