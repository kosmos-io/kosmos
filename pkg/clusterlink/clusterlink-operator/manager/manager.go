package manager

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

type ManagerInstaller struct {
}

func New() *ManagerInstaller {
	return &ManagerInstaller{}
}

const (
	ResourceName = "clusterlink-controller-manager"
)

func applyServiceAccount(opt *option.AddonOption) error {
	clCtrManagerServiceAccountBytes, err := operatorutils.ParseTemplate(clusterlinkManagerServiceAccount, ServiceAccountReplace{
		Namespace: opt.GetSpecNamespace(),
		Name:      ResourceName,
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink controller-manager serviceaccount template :%v", err)
	}

	clCtrManagerServiceAccount := &corev1.ServiceAccount{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clCtrManagerServiceAccountBytes, clCtrManagerServiceAccount); err != nil {
		return fmt.Errorf("decode controller-manager serviceaccount error: %v", err)
	}

	if err := operatorutils.CreateOrUpdateServiceAccount(opt.KubeClientSet, clCtrManagerServiceAccount); err != nil {
		return fmt.Errorf("create clusterlink agent serviceaccount error: %v", err)
	}

	// TODO: wati

	return nil
}

func applyDeployment(opt *option.AddonOption) error {
	clCtrManagerDeploymentBytes, err := operatorutils.ParseTemplate(clusterlinkManagerDeployment, DeploymentReplace{
		Namespace:          opt.GetSpecNamespace(),
		Name:               ResourceName,
		ProxyConfigMapName: kosmosutils.ProxySecretName,
		ClusterName:        opt.GetName(),
		ImageRepository:    opt.GetImageRepository(),
		Version:            opt.Version,
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink controller-manager deployment template :%v", err)
	}

	if clCtrManagerDeploymentBytes == nil {
		return fmt.Errorf("wait klusterlink controller-manager deployment  timeout")
	}

	clCtrManagerDeployment := &appsv1.Deployment{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clCtrManagerDeploymentBytes, clCtrManagerDeployment); err != nil {
		return fmt.Errorf("decode controller-manager deployment error: %v", err)
	}

	if err := operatorutils.CreateOrUpdateDeployment(opt.KubeClientSet, clCtrManagerDeployment); err != nil {
		return fmt.Errorf("create clusterlink controller-manager deployment error: %v", err)
	}

	// TODO: wati

	return nil
}

func applyClusterRole(opt *option.AddonOption) error {
	clCtrManagerClusterRoleBytes, err := operatorutils.ParseTemplate(clusterlinkManagerClusterRole, ClusterRoleReplace{
		Name: ResourceName,
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink controller-manager clusterrole template :%v", err)
	}

	if clCtrManagerClusterRoleBytes == nil {
		return fmt.Errorf("wait klusterlink controller-manager clusterrole  timeout")
	}

	clCtrManagerClusterRole := &rbacv1.ClusterRole{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clCtrManagerClusterRoleBytes, clCtrManagerClusterRole); err != nil {
		return fmt.Errorf("decode controller-manager clusterrole error: %v", err)
	}

	if err := operatorutils.CreateOrUpdateClusterRole(opt.KubeClientSet, clCtrManagerClusterRole); err != nil {
		return fmt.Errorf("create clusterlink controller-manager clusterrole error: %v", err)
	}

	// TODO: wati

	return nil
}

func applyClusterRoleBinding(opt *option.AddonOption) error {
	clCtrManagerClusterRoleBindingBytes, err := operatorutils.ParseTemplate(clusterlinkManagerClusterRoleBinding, ClusterRoleBindingReplace{
		Name:      ResourceName,
		Namespace: opt.GetSpecNamespace(),
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink controller-manager clusterrolebinding template :%v", err)
	}

	if clCtrManagerClusterRoleBindingBytes == nil {
		return fmt.Errorf("wait klusterlink controller-manager clusterrolebinding  timeout")
	}

	clCtrManagerClusterRoleBinding := &rbacv1.ClusterRoleBinding{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clCtrManagerClusterRoleBindingBytes, clCtrManagerClusterRoleBinding); err != nil {
		return fmt.Errorf("decode controller-manager clusterrolebinding error: %v", err)
	}

	if err := operatorutils.CreateOrUpdateClusterRoleBinding(opt.KubeClientSet, clCtrManagerClusterRoleBinding); err != nil {
		return fmt.Errorf("create clusterlink controller-manager clusterrolebinding error: %v", err)
	}

	// TODO: wati

	return nil
}

// Install resources related to CR:cluster
func (i *ManagerInstaller) Install(opt *option.AddonOption) error {
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
	return nil
}

// Uninstall resources related to CR:cluster
func (i *ManagerInstaller) Uninstall(opt *option.AddonOption) error {
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

	klog.Infof("Uninstall clusterlink controller-manager on cluster successfully")
	return nil
}
