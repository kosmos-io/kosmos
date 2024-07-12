package controlplane

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/virtualcluster"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureApiServerExternalEndPoint(dynamicClient dynamic.Interface) error {
	err := installApiServerExternalEndpointInVirtualCluster(dynamicClient)
	if err != nil {
		return err
	}

	err = installApiServerExternalServiceInVirtualCluster(dynamicClient)
	if err != nil {
		return err
	}
	return nil
}

func installApiServerExternalEndpointInVirtualCluster(dynamicClient dynamic.Interface) error {
	klog.V(4).Info("begin to get kubernetes endpoint")
	kubeEndpointUnstructured, err := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "endpoints",
	}).Namespace(constants.DefaultNs).Get(context.TODO(), "kubernetes", metav1.GetOptions{})
	if err != nil {
		klog.Error("get Kubernetes endpoint failed", err)
		return errors.Wrap(err, "failed to get kubernetes endpoint")
	}
	klog.V(4).Info("the Kubernetes endpoint is：", kubeEndpointUnstructured)

	if kubeEndpointUnstructured != nil {
		kubeEndpoint := &corev1.Endpoints{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(kubeEndpointUnstructured.Object, kubeEndpoint)
		if err != nil {
			klog.Error("switch Kubernetes endpoint to typed object failed", err)
			return errors.Wrap(err, "failed to convert kubernetes endpoint to typed object")
		}

		newEndpoint := kubeEndpoint.DeepCopy()
		newEndpoint.Name = constants.ApiServerExternalService
		newEndpoint.Namespace = constants.DefaultNs
		newEndpoint.ResourceVersion = ""
		newEndpointUnstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(newEndpoint)
		if err != nil {
			klog.Error("switch new endpoint to unstructured object failed", err)
			return errors.Wrap(err, "failed to convert new endpoint to unstructured object")
		}
		klog.V(4).Info("after switch the Endpoint unstructured is：", newEndpointUnstructuredObj)

		newEndpointUnstructured := &unstructured.Unstructured{Object: newEndpointUnstructuredObj}
		createResult, err := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "endpoints",
		}).Namespace(constants.DefaultNs).Create(context.TODO(), newEndpointUnstructured, metav1.CreateOptions{})
		if err != nil {
			klog.Error("create api-server-external-service endpoint failed", err)
			return errors.Wrap(err, "failed to create api-server-external-service endpoint")
		} else {
			klog.V(4).Info("success create api-server-external-service endpoint:", createResult)
		}
	} else {
		return errors.New("kubernetes endpoint does not exist")
	}

	return nil
}

func installApiServerExternalServiceInVirtualCluster(dynamicClient dynamic.Interface) error {
	port, err := getEndPointPort(dynamicClient)
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

	var obj unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(apiServerExternalServiceBytes), &obj); err != nil {
		return fmt.Errorf("err when decoding api-server-external service in virtual cluster: %w", err)
	}

	err = util.CreateObject(dynamicClient, "default", "api-server-external-service", &obj)
	if err != nil {
		return fmt.Errorf("error when creating api-server-external service in virtual cluster err: %w", err)
	}
	return nil
}

func getEndPointPort(dynamicClient dynamic.Interface) (int32, error) {
	klog.V(4).Info("begin to get Endpoints ports...")
	endpointsRes := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "endpoints",
	}).Namespace(constants.DefaultNs)

	endpointsRaw, err := endpointsRes.Get(context.TODO(), constants.ApiServerExternalService, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get Endpoints failed: %v", err)
		return 0, err
	}

	subsets, found, err := unstructured.NestedSlice(endpointsRaw.Object, "subsets")
	if !found || err != nil {
		klog.Errorf("The subsets field was not found or parsing error occurred: %v", err)
		return 0, fmt.Errorf("subsets field not found or error parsing it")
	}

	if len(subsets) == 0 {
		klog.Errorf("subsets is empty")
		return 0, fmt.Errorf("No subsets found in the endpoints")
	}

	subset := subsets[0].(map[string]interface{})
	ports, found, err := unstructured.NestedSlice(subset, "ports")
	if !found || err != nil {
		klog.Errorf("ports field not found or parsing error: %v", err)
		return 0, fmt.Errorf("ports field not found or error parsing it")
	}

	if len(ports) == 0 {
		klog.Errorf("Port not found in the endpoint")
		return 0, fmt.Errorf("No ports found in the endpoint")
	}

	port := ports[0].(map[string]interface{})
	portNum, found, err := unstructured.NestedInt64(port, "port")
	if !found || err != nil {
		klog.Errorf("ports field not found or parsing error: %v", err)
		return 0, fmt.Errorf("port field not found or error parsing it")
	}

	klog.V(4).Infof("The port number was successfully obtained: %d", portNum)
	return int32(portNum), nil
}
