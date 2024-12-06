package controlplane

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/common"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/virtualcluster"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type IPFamilies struct {
	IPv4 bool
	IPv6 bool
}

func EnsureAPIServerExternalEndPoint(kubeClient kubernetes.Interface, resource common.Resource) error {
	err := EnsureKosmosSystemNamespace(kubeClient)
	if err != nil {
		return err
	}

	err = CreateOrUpdateAPIServerExternalEndpoint(kubeClient, resource)
	if err != nil {
		return err
	}

	err = CreateOrUpdateAPIServerExternalService(kubeClient)
	if err != nil {
		return err
	}
	return nil
}

func CreateOrUpdateAPIServerExternalEndpoint(kubeClient kubernetes.Interface, resource common.Resource) error {
	klog.V(4).Info("begin to create or update api-server-external-service endpoint")
	// 获取API Server 所在的节点信息
	nodes, err := getAPIServerNodes(resource.RootClientSet, resource.Namespace)
	if err != nil {
		return fmt.Errorf("failed to get API server nodes: %w", err)
	}
	if len(nodes.Items) == 0 {
		return fmt.Errorf("no API server nodes found in the cluster")
	}
	// 收集API Server节点的InternalIp地址
	var addresses []corev1.EndpointAddress
	for _, node := range nodes.Items {
		klog.V(4).Infof("API server node: %s", node.Name)
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				klog.V(4).Infof("Node internal IP: %s", address.Address)
				addresses = append(addresses, corev1.EndpointAddress{
					IP: address.Address,
				})
			}
		}
	}

	if len(addresses) == 0 {
		return fmt.Errorf("no internal IP addresses found for the API server nodes")
	}

	// 获取API Server 的端口信息
	apiServerPort, ok := resource.Vc.Status.PortMap[constants.APIServerPortKey]
	if !ok {
		return fmt.Errorf("failed to get API server port from VirtualCluster status")
	}
	klog.V(4).Infof("API server port: %d", apiServerPort)

	// 创建或更新 api-server-external-service Endpoint
	endpoint := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.APIServerExternalService,
			Namespace: constants.KosmosNs,
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: addresses,
				Ports: []corev1.EndpointPort{
					{
						Name:     "https",
						Port:     apiServerPort,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		},
	}

	_, err = kubeClient.CoreV1().Endpoints(constants.KosmosNs).Get(context.TODO(), constants.APIServerExternalService, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// 创建 Endpoint
			_, err = kubeClient.CoreV1().Endpoints(constants.KosmosNs).Create(context.TODO(), endpoint, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create api-server-external-service endpoint: %w", err)
			}
			klog.V(4).Info("api-server-external-service endpoint created successfully")
		} else {
			return fmt.Errorf("failed to get api-server-external-service endpoint: %w", err)
		}
	} else {
		// 更新 Endpoint
		_, err = kubeClient.CoreV1().Endpoints(constants.KosmosNs).Update(context.TODO(), endpoint, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update api-server-external-service endpoint: %w", err)
		}
		klog.V(4).Info("api-server-external-service endpoint updated successfully")
	}

	return nil
}

func CreateOrUpdateAPIServerExternalService(kubeClient kubernetes.Interface) error {
	port, ipFamilies, err := getEndPointInfo(kubeClient)
	if err != nil {
		return fmt.Errorf("error when getEndPointPort: %w", err)
	}
	apiServerExternalServiceBytes, err := util.ParseTemplate(virtualcluster.APIServerExternalService, struct {
		ServicePort int32
		IPv4        bool
		IPv6        bool
	}{
		ServicePort: port,
		IPv4:        ipFamilies.IPv4,
		IPv6:        ipFamilies.IPv6,
	})
	if err != nil {
		return fmt.Errorf("error when parsing api-server-external-serive template: %w", err)
	}

	var svc corev1.Service
	if err := yaml.Unmarshal([]byte(apiServerExternalServiceBytes), &svc); err != nil {
		return fmt.Errorf("err when decoding api-server-external-service in virtual cluster: %w", err)
	}
	_, err = kubeClient.CoreV1().Services(constants.KosmosNs).Get(context.TODO(), constants.APIServerExternalService, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Try to create the service
			_, err = kubeClient.CoreV1().Services(constants.KosmosNs).Create(context.TODO(), &svc, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("error when creating api-server-external-service: %w", err)
			}
			klog.V(4).Info("successfully created api-server-external-service service")
		} else {
			return fmt.Errorf("error when get api-server-external-service: %w", err)
		}
	} else {
		_, err = kubeClient.CoreV1().Services(constants.KosmosNs).Update(context.TODO(), &svc, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error when updating api-server-external-service: %w", err)
		}
		klog.V(4).Info("successfully updated api-server-external-service service")
	}

	return nil
}

func getEndPointInfo(kubeClient kubernetes.Interface) (int32, IPFamilies, error) {
	klog.V(4).Info("begin to get Endpoints ports...")
	endpoints, err := kubeClient.CoreV1().Endpoints(constants.KosmosNs).Get(context.TODO(), constants.APIServerExternalService, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get Endpoints failed: %v", err)
		return 0, IPFamilies{}, err
	}

	if len(endpoints.Subsets) == 0 {
		klog.Errorf("subsets is empty")
		return 0, IPFamilies{}, fmt.Errorf("No subsets found in the endpoints")
	}

	subset := endpoints.Subsets[0]

	if len(subset.Ports) == 0 {
		klog.Errorf("Port not found in the endpoint")
		return 0, IPFamilies{}, fmt.Errorf("No ports found in the endpoint")
	}

	port := subset.Ports[0].Port
	klog.V(4).Infof("The port number was successfully obtained: %d", port)

	ipFamilies := IPFamilies{
		IPv4: false,
		IPv6: false,
	}

	// Check if the addresses contain IPv4 or IPv6
	for _, address := range subset.Addresses {
		if utils.IsIPv4(address.IP) {
			ipFamilies.IPv4 = true
		}
		if utils.IsIPv6(address.IP) {
			ipFamilies.IPv6 = true
		}
	}

	klog.V(4).Infof("IPv4: %v, IPv6: %v", ipFamilies.IPv4, ipFamilies.IPv6)

	return port, ipFamilies, nil
}

func EnsureKosmosSystemNamespace(kubeClient kubernetes.Interface) error {
	// check if kosmos-system namespace exists
	_, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), constants.KosmosNs, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: constants.KosmosNs,
				},
			}
			_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create kosmos-system namespace: %v", err)
			}
			fmt.Println("Created kosmos-system namespace")
			return nil
		}

		return fmt.Errorf("failed to get kosmos-system namespace: %v", err)
	}
	// 命名空间已存在
	return nil
}

func getAPIServerNodes(rootClientSet kubernetes.Interface, namespace string) (*corev1.NodeList, error) {
	klog.V(4).Info("begin to get API server nodes")
	// 获取带有 virtualCluster-app=apiserver 标签的 kube-apiserver Pod
	apiServerPods, err := rootClientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "virtualCluster-app=apiserver",
	})
	if err != nil {
		klog.Errorf("failed to list kube-apiserver pod: %v", err)
		return nil, errors.Wrap(err, "failed to list kube-apiserver pods")
	}
	// 收集所有 API Server Pod 所在的节点名称
	var nodeNames []string
	for _, pod := range apiServerPods.Items {
		klog.V(4).Infof("API server pod %s is on node: %s", pod.Name, pod.Spec.NodeName)
		nodeNames = append(nodeNames, pod.Spec.NodeName)
	}

	if len(nodeNames) == 0 {
		klog.Errorf("no API server pods found in the namespace")
		return nil, fmt.Errorf("no API server pods found")
	}

	// 查询每个节点并收集节点信息
	var nodesList []corev1.Node
	for _, nodeName := range nodeNames {
		node, err := rootClientSet.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("failed to get node %s: %v", nodeName, err)
			return nil, fmt.Errorf("failed to get node %s: %v", nodeName, err)
		}
		klog.V(4).Infof("Found node: %s", node.Name)
		nodesList = append(nodesList, *node)
	}

	nodes := &corev1.NodeList{
		Items: nodesList,
	}

	klog.V(4).Infof("got %d API server nodes", len(nodes.Items))

	if len(nodes.Items) == 0 {
		klog.Errorf("no nodes found for the API server pods")
		return nil, fmt.Errorf("no nodes found for the API server pods")
	}

	return nodes, nil
}
