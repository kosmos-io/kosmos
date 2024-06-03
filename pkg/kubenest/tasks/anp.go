package tasks

import (
	"context"
	"fmt"
	"strings"

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

	ko "github.com/kosmos.io/kosmos/cmd/kubenest/operator/app/options"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/apiserver"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	apiclient "github.com/kosmos.io/kosmos/pkg/kubenest/util/api-client"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewAnpTask() workflow.Task {
	return workflow.Task{
		Name:        "anp",
		Run:         runAnp,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "Upload-ProxyAgentCert",
				Run:  runUploadProxyAgentCert,
			},
			{
				Name: "deploy-anp-server",
				Run:  runAnpServer,
			},
			{
				Name: "deploy-anp-agent",
				Run:  runAnpAgent,
			},
			{
				Name: "check-anp-health",
				Run:  runCheckVirtualClusterAnp,
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
	name, namespace := data.GetName(), data.GetNamespace()
	kubeNestOpt := data.KubeNestOpt()
	portMap := data.HostPortMap()
	// install egress_selector_configuration config map
	egressSelectorConfig, err := util.ParseTemplate(apiserver.EgressSelectorConfiguration, struct {
		Namespace       string
		Name            string
		AnpMode         string
		ProxyServerPort int32
		SvcName         string
	}{
		Namespace:       namespace,
		Name:            name,
		ProxyServerPort: portMap[constants.ApiServerNetworkProxyServerPortKey],
		SvcName:         fmt.Sprintf("%s-konnectivity-server.%s.svc.cluster.local", name, namespace),
		AnpMode:         kubeNestOpt.AnpMode,
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
	err = installAnpServer(data.RemoteClient(), name, namespace, portMap, kubeNestOpt)
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
	return installAnpAgent(data)
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
	name, namespace := data.GetName(), data.GetNamespace()
	client := data.RemoteClient()
	portMap := data.HostPortMap()
	kubeNestOpt := data.KubeNestOpt()
	anpManifest, vcClient, err := getAnpAgentManifest(client, name, namespace, portMap, kubeNestOpt)
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
func installAnpServer(client clientset.Interface, name, namespace string, portMap map[string]int32, kubeNestOpt *ko.KubeNestOptions) error {
	imageRepository, imageVersion := util.GetImageMessage()
	clusterIp, err := util.GetEtcdServiceClusterIp(namespace, name+constants.EtcdSuffix, client)
	if err != nil {
		return nil
	}

	apiserverDeploymentBytes, err := util.ParseTemplate(apiserver.ApiserverAnpDeployment, struct {
		DeploymentName, Namespace, ImageRepository, EtcdClientService, Version string
		ServiceSubnet, VirtualClusterCertsSecret, EtcdCertsSecret              string
		Replicas                                                               int
		EtcdListenClientPort                                                   int32
		ClusterPort                                                            int32
		AgentPort                                                              int32
		ServerPort                                                             int32
		HealthPort                                                             int32
		AdminPort                                                              int32
		KubeconfigSecret                                                       string
		Name                                                                   string
		AnpMode                                                                string
		AdmissionPlugins                                                       bool
	}{
		DeploymentName:            fmt.Sprintf("%s-%s", name, "apiserver"),
		Namespace:                 namespace,
		ImageRepository:           imageRepository,
		Version:                   imageVersion,
		EtcdClientService:         clusterIp,
		ServiceSubnet:             constants.ApiServerServiceSubnet,
		VirtualClusterCertsSecret: fmt.Sprintf("%s-%s", name, "cert"),
		EtcdCertsSecret:           fmt.Sprintf("%s-%s", name, "etcd-cert"),
		Replicas:                  kubeNestOpt.ApiServerReplicas,
		EtcdListenClientPort:      constants.ApiServerEtcdListenClientPort,
		ClusterPort:               portMap[constants.ApiServerPortKey],
		AgentPort:                 portMap[constants.ApiServerNetworkProxyAgentPortKey],
		ServerPort:                portMap[constants.ApiServerNetworkProxyServerPortKey],
		HealthPort:                portMap[constants.ApiServerNetworkProxyHealthPortKey],
		AdminPort:                 portMap[constants.ApiServerNetworkProxyAdminPortKey],
		KubeconfigSecret:          fmt.Sprintf("%s-%s", name, "admin-config-clusterip"),
		Name:                      name,
		AnpMode:                   kubeNestOpt.AnpMode,
		AdmissionPlugins:          kubeNestOpt.AdmissionPlugins,
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

func installAnpAgent(data InitData) error {
	client := data.RemoteClient()
	name := data.GetName()
	namespace := data.GetNamespace()
	portMap := data.HostPortMap()
	kubeNestOpt := data.KubeNestOpt()
	anpAgentManifestBytes, vcClient, err2 := getAnpAgentManifest(client, name, namespace, portMap, kubeNestOpt)
	if err2 != nil {
		return err2
	}
	actionFunc := func(ctx context.Context, c dynamic.Interface, u *unstructured.Unstructured) error {
		// create the object
		return util.ApplyObject(vcClient, u)
	}
	return util.ForEachObjectInYAML(context.TODO(), vcClient, []byte(anpAgentManifestBytes), "", actionFunc)
}

func getAnpAgentManifest(client clientset.Interface, name string, namespace string, portMap map[string]int32, kubeNestOpt *ko.KubeNestOptions) (string, dynamic.Interface, error) {
	imageRepository, imageVersion := util.GetImageMessage()
	// get apiServer hostIp
	proxyServerHost, err := getDeploymentPodIPs(client, namespace, fmt.Sprintf("%s-%s", name, "apiserver"))
	if err != nil {
		klog.Warningf("Failed to get apiserver hostIp, err: %v", err)
		// ignore if can't get the hostIp when uninstall  the deployment
		proxyServerHost = []string{"127.0.0.1"}
	}

	anpAgentManifeattBytes, err := util.ParseTemplate(apiserver.AnpAgentManifest, struct {
		ImageRepository string
		Version         string
		AgentPort       int32
		ProxyServerHost []string
		AnpMode         string
		AgentCertName   string
	}{
		ImageRepository: imageRepository,
		Version:         imageVersion,
		AgentPort:       portMap[constants.ApiServerNetworkProxyAgentPortKey],
		ProxyServerHost: proxyServerHost,
		AnpMode:         kubeNestOpt.AnpMode,
		AgentCertName:   fmt.Sprintf("%s-%s", name, "cert"),
	})
	if err != nil {
		return "", nil, fmt.Errorf("error when parsing virtual cluster apiserver deployment template: %w", err)
	}
	klog.V(4).InfoS("[anp] apply anp agent", "agent manifest", anpAgentManifeattBytes)
	vcClient, err := getVcDynamicClient(client, name, namespace)
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

func getVcDynamicClient(client clientset.Interface, name, namespace string) (dynamic.Interface, error) {
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
func getVcClientset(client clientset.Interface, name, namespace string) (clientset.Interface, error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(),
		fmt.Sprintf("%s-%s", name, constants.AdminConfig), metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Get virtualcluster kubeconfig secret error")
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(secret.Data[constants.KubeConfig])
	if err != nil {
		return nil, err
	}

	vcClient, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return vcClient, nil
}

func runUploadProxyAgentCert(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("upload proxy agent cert task invoked with an invalid data struct")
	}
	name, namespace := data.GetName(), data.GetNamespace()
	certList := data.CertList()
	certsData := make(map[string][]byte, len(certList))
	for _, c := range certList {
		// only upload apisever cert
		if strings.Contains(c.KeyName(), "apiserver") {
			certsData[c.KeyName()] = c.KeyData()
			certsData[c.CertName()] = c.CertData()
		}
	}
	vcClient, err := getVcClientset(data.RemoteClient(), name, namespace)
	if err != nil {
		return fmt.Errorf("failed to get virtual cluster client, err: %w", err)
	}
	err = createOrUpdateSecret(vcClient, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", data.GetName(), "cert"),
			Namespace: "kube-system",
			Labels:    VirtualClusterControllerLabel,
		},
		Data: certsData,
	})
	if err != nil {
		return fmt.Errorf("failed to upload agent cert to tenant, err: %w", err)
	}

	klog.V(2).InfoS("[Upload-ProxyAgentCert] Successfully uploaded virtual cluster agent certs to secret", "virtual cluster", klog.KObj(data))
	return nil
}

func runCheckVirtualClusterAnp(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("check-VirtualClusterAnp task invoked with an invalid data struct")
	}

	checker := apiclient.NewVirtualClusterChecker(data.RemoteClient(), constants.ComponentBeReadyTimeout)

	err := checker.WaitForSomePods(virtualClusterAnpLabels.String(), data.GetNamespace(), 1)
	if err != nil {
		return fmt.Errorf("checking for virtual cluster anp to ready timeout, err: %w", err)
	}

	klog.V(2).InfoS("[check-VirtualClusterAPIServer] the virtual cluster anp is ready", "virtual cluster", klog.KObj(data))
	return nil
}
