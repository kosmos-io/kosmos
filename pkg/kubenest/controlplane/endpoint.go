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

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/virtualcluster"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureApiServerExternalEndPoint(kubeClient kubernetes.Interface) error {
	err := CreateOrUpdateApiServerExternalEndpoint(kubeClient)
	if err != nil {
		return err
	}

	err = CreateOrUpdateApiServerExternalService(kubeClient)
	if err != nil {
		return err
	}
	return nil
}

func CreateOrUpdateApiServerExternalEndpoint(kubeClient kubernetes.Interface) error {
	klog.V(4).Info("begin to get kubernetes endpoint")
	kubeEndpoint, err := kubeClient.CoreV1().Endpoints(constants.DefaultNs).Get(context.TODO(), "kubernetes", metav1.GetOptions{})
	if err != nil {
		klog.Error("get Kubernetes endpoint failed", err)
		return errors.Wrap(err, "failed to get kubernetes endpoint")
	}
	klog.V(4).Info("the Kubernetes endpoint is：", kubeEndpoint)

	newEndpoint := kubeEndpoint.DeepCopy()
	newEndpoint.Name = constants.ApiServerExternalService
	newEndpoint.Namespace = constants.DefaultNs
	newEndpoint.ResourceVersion = ""

	// Reconstruct the Ports without the 'name' field
	for i := range newEndpoint.Subsets {
		for j := range newEndpoint.Subsets[i].Ports {
			newEndpoint.Subsets[i].Ports[j] = corev1.EndpointPort{
				Port:     newEndpoint.Subsets[i].Ports[j].Port,
				Protocol: newEndpoint.Subsets[i].Ports[j].Protocol,
			}
		}
	}

	// Try to create the endpoint
	_, err = kubeClient.CoreV1().Endpoints(constants.DefaultNs).Create(context.TODO(), newEndpoint, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			klog.Error("create api-server-external-service endpoint failed", err)
			return errors.Wrap(err, "failed to create api-server-external-service endpoint")
		}

		// Endpoint already exists, retrieve it
		existingEndpoint, err := kubeClient.CoreV1().Endpoints(constants.DefaultNs).Get(context.TODO(), constants.ApiServerExternalService, metav1.GetOptions{})
		if err != nil {
			klog.Error("get existing api-server-external-service endpoint failed", err)
			return errors.Wrap(err, "failed to get existing api-server-external-service endpoint")
		}

		// Update the existing endpoint
		newEndpoint.SetResourceVersion(existingEndpoint.ResourceVersion)
		newEndpoint.SetUID(existingEndpoint.UID)
		_, err = kubeClient.CoreV1().Endpoints(constants.DefaultNs).Update(context.TODO(), newEndpoint, metav1.UpdateOptions{})
		if err != nil {
			klog.Error("update api-server-external-service endpoint failed", err)
			return errors.Wrap(err, "failed to update api-server-external-service endpoint")
		} else {
			klog.V(4).Info("successfully updated api-server-external-service endpoint")
		}
	} else {
		klog.V(4).Info("successfully created api-server-external-service endpoint")
	}

	return nil
}

func CreateOrUpdateApiServerExternalService(kubeClient kubernetes.Interface) error {
	port, err := getEndPointPort(kubeClient)
	if err != nil {
		return fmt.Errorf("error when getEndPointPort: %w", err)
	}
	apiServerExternalServiceBytes, err := util.ParseTemplate(virtualcluster.ApiServerExternalService, struct {
		ServicePort int32
	}{
		ServicePort: port,
	})
	if err != nil {
		return fmt.Errorf("error when parsing api-server-external-serive template: %w", err)
	}

	var svc corev1.Service
	if err := yaml.Unmarshal([]byte(apiServerExternalServiceBytes), &svc); err != nil {
		return fmt.Errorf("err when decoding api-server-external-service in virtual cluster: %w", err)
	}
	_, err = kubeClient.CoreV1().Services(constants.DefaultNs).Get(context.TODO(), constants.ApiServerExternalService, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			// Try to create the service
			_, err = kubeClient.CoreV1().Services(constants.DefaultNs).Create(context.TODO(), &svc, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("error when creating api-server-external-service: %w", err)
			}
		}
	}
	klog.V(4).Info("successfully created api-server-external-service service")
	return nil
}

func getEndPointPort(kubeClient kubernetes.Interface) (int32, error) {
	klog.V(4).Info("begin to get Endpoints ports...")
	endpoints, err := kubeClient.CoreV1().Endpoints(constants.DefaultNs).Get(context.TODO(), constants.ApiServerExternalService, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get Endpoints failed: %v", err)
		return 0, err
	}

	if len(endpoints.Subsets) == 0 {
		klog.Errorf("subsets is empty")
		return 0, fmt.Errorf("No subsets found in the endpoints")
	}

	subset := endpoints.Subsets[0]
	if len(subset.Ports) == 0 {
		klog.Errorf("Port not found in the endpoint")
		return 0, fmt.Errorf("No ports found in the endpoint")
	}

	port := subset.Ports[0].Port
	klog.V(4).Infof("The port number was successfully obtained: %d", port)
	return port, nil
}
