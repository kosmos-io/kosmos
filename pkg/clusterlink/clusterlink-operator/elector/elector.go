package elector

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clusterlink/clusterlink-operator/option"
	operatorutils "github.com/kosmos.io/kosmos/pkg/clusterlink/clusterlink-operator/utils"
	kosmosutils "github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	ResourceName = "clusterlink-elector"
)

type ElectorInstaller struct {
}

func New() *ElectorInstaller {
	return &ElectorInstaller{}
}

func applyServiceAccount(opt *option.AddonOption) error {
	clElectorServiceAccountBytes, err := operatorutils.ParseTemplate(clusterlinkElectorServiceAccount, ServiceAccountReplace{
		Namespace: opt.GetSpecNamespace(),
		Name:      ResourceName,
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink elector serviceaccount template :%v", err)
	}

	if clElectorServiceAccountBytes == nil {
		return fmt.Errorf("wait klusterlink elector serviceaccount  timeout")
	}

	clElectorServiceAccount := &corev1.ServiceAccount{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clElectorServiceAccountBytes, clElectorServiceAccount); err != nil {
		return fmt.Errorf("decode elector serviceaccount error: %v", err)
	}

	if err := operatorutils.CreateOrUpdateServiceAccount(opt.KubeClientSet, clElectorServiceAccount); err != nil {
		return fmt.Errorf("create clusterlink agent serviceaccount error: %v", err)
	}

	// TODO: wati

	return nil
}

func applyDeployment(opt *option.AddonOption) error {
	clElectorDeploymentBytes, err := operatorutils.ParseTemplate(clusterlinkElectorDeployment, DeploymentReplace{
		Namespace:          opt.GetSpecNamespace(),
		Name:               ResourceName,
		ClusterName:        opt.GetName(),
		ImageRepository:    opt.GetImageRepository(),
		ProxyConfigMapName: kosmosutils.ProxySecretName,
		Version:            opt.Version,
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink elector deployment template :%v", err)
	}

	if clElectorDeploymentBytes == nil {
		return fmt.Errorf("wait klusterlink elector deployment  timeout")
	}

	clElectorDeployment := &appsv1.Deployment{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clElectorDeploymentBytes, clElectorDeployment); err != nil {
		return fmt.Errorf("decode elector deployment error: %v", err)
	}

	if err := operatorutils.CreateOrUpdateDeployment(opt.KubeClientSet, clElectorDeployment); err != nil {
		return fmt.Errorf("create clusterlink elector deployment error: %v", err)
	}

	// TODO: wati

	return nil
}

func applyClusterRole(opt *option.AddonOption) error {
	clElectorClusterRoleBytes, err := operatorutils.ParseTemplate(clusterlinkElectorClusterRole, ClusterRoleReplace{
		Name: ResourceName,
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink elector clusterrole template :%v", err)
	}

	if clElectorClusterRoleBytes == nil {
		return fmt.Errorf("wait klusterlink elector clusterrole  timeout")
	}

	clElectorClusterRole := &rbacv1.ClusterRole{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clElectorClusterRoleBytes, clElectorClusterRole); err != nil {
		return fmt.Errorf("decode elector clusterrole error: %v", err)
	}

	if err := operatorutils.CreateOrUpdateClusterRole(opt.KubeClientSet, clElectorClusterRole); err != nil {
		return fmt.Errorf("create clusterlink elector clusterrole error: %v", err)
	}

	// TODO: wati

	return nil
}

func applyClusterRoleBinding(opt *option.AddonOption) error {
	clElectorClusterRoleBindingBytes, err := operatorutils.ParseTemplate(clusterlinkElectorClusterRoleBinding, ClusterRoleBindingReplace{
		Name:      ResourceName,
		Namespace: opt.GetSpecNamespace(),
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink elector clusterrolebinding template :%v", err)
	}

	if clElectorClusterRoleBindingBytes == nil {
		return fmt.Errorf("wait klusterlink elector clusterrolebinding  timeout")
	}

	clElectorClusterRoleBinding := &rbacv1.ClusterRoleBinding{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clElectorClusterRoleBindingBytes, clElectorClusterRoleBinding); err != nil {
		return fmt.Errorf("decode elector clusterrolebinding error: %v", err)
	}

	if err := operatorutils.CreateOrUpdateClusterRoleBinding(opt.KubeClientSet, clElectorClusterRoleBinding); err != nil {
		return fmt.Errorf("create clusterlink elector clusterrolebinding error: %v", err)
	}

	// TODO: wati

	return nil
}

// Install resources related to CR:cluster
func (i *ElectorInstaller) Install(opt *option.AddonOption) error {
	if err := applyServiceAccount(opt); err != nil {
		return err
	}

	if err := applyDeployment(opt); err != nil {
		return err
	}

	if err := applyClusterRole(opt); err != nil {
		return err
	}

	if err := applyClusterRoleBinding(opt); err != nil {
		return err
	}

	klog.Infof("Install clusterlink elector on cluster successfully")
	return nil
}

// Uninstall resources related to CR:cluster
func (i *ElectorInstaller) Uninstall(opt *option.AddonOption) error {
	deploymentClient := opt.KubeClientSet.AppsV1().Deployments(opt.GetSpecNamespace())
	if err := deploymentClient.Delete(context.TODO(), ResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	serviceAccountClient := opt.KubeClientSet.CoreV1().ServiceAccounts(opt.GetSpecNamespace())
	if err := serviceAccountClient.Delete(context.TODO(), ResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	clusterRoleBindingClient := opt.KubeClientSet.RbacV1().ClusterRoleBindings()
	if err := clusterRoleBindingClient.Delete(context.TODO(), ResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	clusterRoleClient := opt.KubeClientSet.RbacV1().ClusterRoles()
	if err := clusterRoleClient.Delete(context.TODO(), ResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	klog.Infof("Uninstall clusterlink elector on cluster successfully")
	return nil
}
