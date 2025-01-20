package controlplane

import (
	"context"
	"fmt"

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

func EnsureAPIServerExternalEndPoint(kubeClient kubernetes.Interface, apiServerExternalResource common.APIServerExternalResource) error {
	err := EnsureKosmosSystemNamespace(kubeClient)
	if err != nil {
		return err
	}

	err = CreateOrUpdateAPIServerExternalEndpoint(kubeClient, apiServerExternalResource)
	if err != nil {
		return err
	}

	err = CreateOrUpdateAPIServerExternalService(kubeClient)
	if err != nil {
		return err
	}
	return nil
}

func CreateOrUpdateAPIServerExternalEndpoint(kubeClient kubernetes.Interface, apiServerExternalResource common.APIServerExternalResource) error {
	klog.V(4).Info("begin to create or update api-server-external-service endpoint")
	nodes, err := util.GetAPIServerNodes(apiServerExternalResource.RootClientSet, apiServerExternalResource.Namespace)
	if err != nil {
		return fmt.Errorf("failed to get API server nodes: %w", err)
	}
	if len(nodes.Items) == 0 {
		return fmt.Errorf("no API server nodes found in the cluster")
	}

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

	apiServerPort, ok := apiServerExternalResource.Vc.Status.PortMap[constants.APIServerPortKey]
	if !ok {
		return fmt.Errorf("failed to get API server port from VirtualCluster status")
	}
	klog.V(4).Infof("API server port: %d", apiServerPort)

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
			_, err = kubeClient.CoreV1().Endpoints(constants.KosmosNs).Create(context.TODO(), endpoint, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create api-server-external-service endpoint: %w", err)
			}
			klog.V(4).Info("api-server-external-service endpoint created successfully")
		} else {
			return fmt.Errorf("failed to get api-server-external-service endpoint: %w", err)
		}
	} else {
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
		IPFamilies  []corev1.IPFamily
	}{
		ServicePort: port,
		IPFamilies:  ipFamilies,
	})
	if err != nil {
		return fmt.Errorf("error when parsing api-server-external-serive template: %w", err)
	}

	var svc corev1.Service
	if err := yaml.Unmarshal([]byte(apiServerExternalServiceBytes), &svc); err != nil {
		return fmt.Errorf("err when decoding api-server-external-service in virtual cluster: %w", err)
	}
	klog.V(4).Infof("create svc %s: %s", constants.APIServerExternalService, apiServerExternalServiceBytes)
	_, err = kubeClient.CoreV1().Services(constants.KosmosNs).Get(context.TODO(), constants.APIServerExternalService, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
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

func getEndPointInfo(kubeClient kubernetes.Interface) (int32, []corev1.IPFamily, error) {
	klog.V(4).Info("begin to get Endpoints ports...")
	ipFamilies := utils.IPFamilyGenerator(constants.APIServerServiceSubnet)
	endpoints, err := kubeClient.CoreV1().Endpoints(constants.KosmosNs).Get(context.TODO(), constants.APIServerExternalService, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get Endpoints failed: %v", err)
		return 0, ipFamilies, err
	}

	if len(endpoints.Subsets) == 0 {
		klog.Errorf("subsets is empty")
		return 0, ipFamilies, fmt.Errorf("No subsets found in the endpoints")
	}

	subset := endpoints.Subsets[0]

	if len(subset.Ports) == 0 {
		klog.Errorf("Port not found in the endpoint")
		return 0, ipFamilies, fmt.Errorf("No ports found in the endpoint")
	}

	port := subset.Ports[0].Port
	klog.V(4).Infof("The port number was successfully obtained: %d", port)
	return port, ipFamilies, nil
}

func EnsureKosmosSystemNamespace(kubeClient kubernetes.Interface) error {
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
			klog.V(4).Info("Created kosmos-system namespace")
			return nil
		}

		return fmt.Errorf("failed to get kosmos-system namespace: %v", err)
	}

	return nil
}
