package ctlmaster

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/util/apiclient"
	"github.com/kosmos.io/clusterlink/pkg/operator/addons/utils"
	"github.com/kosmos.io/clusterlink/pkg/version"
)

var deploymentTemplateNameMap = map[string]string{
	"clusterlink-operator":           clusterlinkOperatorDeployment,
	"clusterlink-controller-manager": clusterlinkControllerDeployment,
}

func (i *CommandInitOption) retriveClusterLinkiDP() ([]KubeResourceInfo, error) {

	DeploymentInK8sList := []KubeResourceInfo{}
	dpDeployedByCL := []string{}
	for k := range deploymentTemplateNameMap {
		dpDeployedByCL = append(dpDeployedByCL, k)
	}
	deployments, err := i.KubeClientSet.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("can't get deployment from kubernetes")
		return nil, err
	}
	for _, deploy := range deployments.Items {
		if apiclient.ContainsInSlice(dpDeployedByCL, deploy.Name) {
			DeploymentInK8sList = append(DeploymentInK8sList, KubeResourceInfo{
				Name:           deploy.Name,
				Namespace:      deploy.Namespace,
				ResourceClient: i.KubeClientSet.AppsV1().Deployments(deploy.Namespace),
				Type:           "deployment",
			})
		}
	}
	return DeploymentInK8sList, nil
}

func (i *CommandInitOption) initClusterlinkDeployment() error {

	dpList, err := i.retriveClusterLinkiDP()
	if err != nil {
		return err
	}
	for _, dp := range dpList {
		if dp.Namespace != i.Namespace {
			klog.Fatalf("deploy %s already installed in namespace %s!", dp.Name, dp.Namespace)
			return fmt.Errorf("deploy %s already installed in namespace %s!", dp.Name, dp.Namespace)
		}
	}

	for deploymentName, deploymentTemplate := range deploymentTemplateNameMap {
		klog.Infof("imgae: %s", i.getImageByAppName(deploymentName))
		err := i.deployClusterlinkDeployment(deploymentTemplate, DeploymentReplace{
			Namespace:      i.Namespace,
			Version:        version.GetReleaseVersion().PatchRelease(),
			Imgae:          i.getImageByAppName(deploymentName),
			DeploymentName: deploymentName,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (i *CommandInitOption) deployClusterlinkDeployment(clusterlinkDeployment string,
	replace DeploymentReplace) error {

	klog.Infof("Create Clusterlink %s Deployment", replace.DeploymentName)

	deploymentBytes, err := utils.ParseTemplate(clusterlinkDeployment, replace)
	if err != nil {
		klog.Errorf("error when parsing clusterlink deployment template :%v", err)
		return fmt.Errorf("error when parsing clusterlink deployment template :%w", err)
	} else if deploymentBytes == nil {
		return fmt.Errorf("deployment template get nil")
	}

	// get deployment struct
	deploymentStruct := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(),
		deploymentBytes, deploymentStruct); err != nil {
		klog.Errorf("decode clusterlink DeploymentBytes error : %v ", err)
		return fmt.Errorf("decode clusterlink DeploymentBytes error : %w ", err)
	}

	// create or update
	if _, err := i.KubeClientSet.AppsV1().Deployments(replace.Namespace).Create(context.TODO(),
		deploymentStruct, metav1.CreateOptions{}); err != nil {

		if !apierrors.IsAlreadyExists(err) {
			klog.Errorf("create Deployment error : %v ", err)
			return fmt.Errorf("create Deployment error : %w", err)
		} else {
			klog.Infof("Resource deployment %s clusterlink already exists, Update it", replace.DeploymentName)
			if _, err := i.KubeClientSet.AppsV1().Deployments(replace.Namespace).Update(context.TODO(),
				deploymentStruct, metav1.UpdateOptions{}); err != nil {
				klog.Errorf("update Deployment error : %v ", err)
				return fmt.Errorf("update Deployment error : %w ", err)
			}
		}
	}

	deploymentLabel := map[string]string{"app": replace.DeploymentName}
	if err := WaitPodReady(i.KubeClientSet, i.Namespace, MapToString(deploymentLabel), 120); err != nil {
		return err
	}
	return nil
}
