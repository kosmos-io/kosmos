package controlplane

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/rbac"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureVirtualSchedulerRBAC(client clientset.Interface, namespace string) error {
	if err := grantVirtualClusterResourceClusterSA(client, namespace); err != nil {
		return err
	}
	if err := grantVirtualClusterResourceClusterRoleBinding(client, namespace); err != nil {
		return err
	}
	if err := grantVirtualClusterResourceClusterRole(client); err != nil {
		return err
	}
	return nil
}

func grantVirtualClusterResourceClusterSA(client clientset.Interface, namespace string) error {
	virtualClusterResourceClusterSABytes, err := util.ParseTemplate(rbac.VirtualSchedulerSA, struct {
		Namespace string
	}{
		Namespace: namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtualCluster-scheduler sa template: %w", err)
	}
	serviceAccount := &v1.ServiceAccount{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(virtualClusterResourceClusterSABytes), serviceAccount); err != nil {
		return fmt.Errorf("err when decoding Karmada view Clusterrole: %w", err)
	}
	return util.CreateOrUpdateClusterSA(client, serviceAccount, namespace)
}

func grantVirtualClusterResourceClusterRoleBinding(client clientset.Interface, namespace string) error {
	virtualClusterResourceClusterRoleBindingBytes, err := util.ParseTemplate(rbac.VirtualSchedulerRoleBinding, struct {
		Namespace string
	}{
		Namespace: namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing virtualCluster-scheduler role binding template: %w", err)
	}
	viewClusterRoleBinding := &rbacv1.ClusterRoleBinding{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(virtualClusterResourceClusterRoleBindingBytes), viewClusterRoleBinding); err != nil {
		return fmt.Errorf("err when decoding virtualCluster scheduler Clusterrole Binding: %w", err)
	}
	return util.CreateOrUpdateClusterRoleBinding(client, viewClusterRoleBinding)
}

func grantVirtualClusterResourceClusterRole(client clientset.Interface) error {
	viewClusterrole := &rbacv1.ClusterRole{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(rbac.VirtualSchedulerRole), viewClusterrole); err != nil {
		return fmt.Errorf("err when decoding virtualCluster scheduler  Clusterrole: %w", err)
	}
	return util.CreateOrUpdateClusterRole(client, viewClusterrole)
}
