package util

import (
	"bytes"
	"fmt"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	DefaultNamespace       = "kosmos-system"
	DefaultImageRepository = "ghcr.io/kosmos-io"
	DefaultInstallModule   = "all"

	ExternalIPPoolNamePrefix = "clusterlink"
	ControlPanelSecretName   = "controlpanel-config"
)

var (
	ClusterGVR = schema.GroupVersionResource{Group: "kosmos.io", Version: "v1alpha1", Resource: "clusters"}
	KnodeGVR   = schema.GroupVersionResource{Group: "kosmos.io", Version: "v1alpha1", Resource: "knodes"}
)

func GenerateDeployment(deployTemplate string, obj interface{}) (*appsv1.Deployment, error) {
	deployBytes, err := parseTemplate(deployTemplate, obj)
	if err != nil {
		return nil, fmt.Errorf("kosmosctl parsing Deployment template exception, error: %v", err)
	} else if deployBytes == nil {
		return nil, fmt.Errorf("kosmosctl get Deployment template exception, value is empty")
	}

	deployStruct := &appsv1.Deployment{}

	if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), deployBytes, deployStruct); err != nil {
		return nil, fmt.Errorf("kosmosctl decode deployBytes error: %v", err)
	}

	return deployStruct, nil
}

func GenerateDaemonSet(dsTemplate string, obj interface{}) (*appsv1.DaemonSet, error) {
	dsBytes, err := parseTemplate(dsTemplate, obj)
	if err != nil {
		return nil, fmt.Errorf("kosmosctl parsing DaemonSet template exception, error: %v", err)
	} else if dsBytes == nil {
		return nil, fmt.Errorf("kosmosctl get DaemonSet template exception, value is empty")
	}

	dsStruct := &appsv1.DaemonSet{}

	if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), dsBytes, dsStruct); err != nil {
		return nil, fmt.Errorf("kosmosctl decode dsBytes error: %v", err)
	}

	return dsStruct, nil
}

func GenerateServiceAccount(saTemplate string, obj interface{}) (*corev1.ServiceAccount, error) {
	saBytes, err := parseTemplate(saTemplate, obj)
	if err != nil {
		return nil, fmt.Errorf("kosmosctl parsing ServiceAccount template exception, error: %v", err)
	} else if saBytes == nil {
		return nil, fmt.Errorf("kosmosctl get ServiceAccount template exception, value is empty")
	}

	saStruct := &corev1.ServiceAccount{}

	if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), saBytes, saStruct); err != nil {
		return nil, fmt.Errorf("kosmosctl decode saBytes error: %v", err)
	}

	return saStruct, nil
}

func GenerateClusterRole(crTemplate string, obj interface{}) (*rbacv1.ClusterRole, error) {
	crBytes, err := parseTemplate(crTemplate, obj)
	if err != nil {
		return nil, fmt.Errorf("kosmosctl parsing ClusterRole template exception, error: %v", err)
	} else if crBytes == nil {
		return nil, fmt.Errorf("kosmosctl get ClusterRole template exception, value is empty")
	}

	crStruct := &rbacv1.ClusterRole{}

	if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), crBytes, crStruct); err != nil {
		return nil, fmt.Errorf("kosmosctl decode crBytes error: %v", err)
	}

	return crStruct, nil
}

func GenerateClusterRoleBinding(crbTemplate string, obj interface{}) (*rbacv1.ClusterRoleBinding, error) {
	crbBytes, err := parseTemplate(crbTemplate, obj)
	if err != nil {
		return nil, fmt.Errorf("kosmosctl parsing ClusterRoleBinding template exception, error: %v", err)
	} else if crbBytes == nil {
		return nil, fmt.Errorf("kosmosctl get ClusterRoleBinding template exception, value is empty")
	}

	crbStruct := &rbacv1.ClusterRoleBinding{}

	if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), crbBytes, crbStruct); err != nil {
		return nil, fmt.Errorf("kosmosctl decode crbBytes error: %v", err)
	}

	return crbStruct, nil
}

func GenerateCustomResourceDefinition(crdTemplate string, obj interface{}) (*apiextensionsv1.CustomResourceDefinition, error) {
	crdBytes, err := parseTemplate(crdTemplate, obj)
	if err != nil {
		return nil, fmt.Errorf("kosmosctl parsing CustomResourceDefinition template exception, error: %v", err)
	} else if crdBytes == nil {
		return nil, fmt.Errorf("kosmosctl get CustomResourceDefinition template exception, value is empty")
	}

	crdStruct := &apiextensionsv1.CustomResourceDefinition{}

	if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), crdBytes, crdStruct); err != nil {
		return nil, fmt.Errorf("kosmosctl decode crdBytes error: %v", err)
	}

	return crdStruct, nil
}

func parseTemplate(strTmpl string, obj interface{}) ([]byte, error) {
	var buf bytes.Buffer
	tmpl, err := template.New("template").Parse(strTmpl)
	if err != nil {
		return nil, fmt.Errorf("error when parsing template: %v", err)
	}
	err = tmpl.Execute(&buf, obj)
	if err != nil {
		return nil, fmt.Errorf("error when executing template: %v", err)
	}
	return buf.Bytes(), nil
}
