package util

import (
	"bytes"
	"fmt"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

func GenerateDeployment(deployTemplate string, obj interface{}) (*appsv1.Deployment, error) {
	deployBytes, err := ParseTemplate(deployTemplate, obj)
	if err != nil {
		return nil, fmt.Errorf("error when parsing kosmos crd template :%v", err)
	} else if deployBytes == nil {
		return nil, fmt.Errorf("kosmos deployment template get nil")
	}

	deployStruct := &appsv1.Deployment{}

	if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), deployBytes, deployStruct); err != nil {
		return nil, fmt.Errorf("decode kosmos deployBytes error : %v ", err)
	}

	return deployStruct, nil
}

func GenerateCustomResourceDefinition(crdTemplate string, obj interface{}) (*apiextensionsv1.CustomResourceDefinition, error) {
	crdBytes, err := ParseTemplate(crdTemplate, obj)
	if err != nil {
		return nil, fmt.Errorf("error when parsing kosmos crd template :%v", err)
	} else if crdBytes == nil {
		return nil, fmt.Errorf("kosmos crd template get nil")
	}

	crdStruct := &apiextensionsv1.CustomResourceDefinition{}

	if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), crdBytes, crdStruct); err != nil {
		return nil, fmt.Errorf("decode kosmos crdBytes error : %v ", err)
	}

	return crdStruct, nil
}

func ParseTemplate(strTmpl string, obj interface{}) ([]byte, error) {
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
