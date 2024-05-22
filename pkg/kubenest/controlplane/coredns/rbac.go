package coredns

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/kubenest/manifest/controlplane/coredns/host"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

func EnsureCoreDnsRBAC(client clientset.Interface, namespace string, name string) error {
	if err := grantCoreDnsClusterSA(client, namespace); err != nil {
		return err
	}
	if err := grantCoreDnsClusterRoleBinding(client, namespace, name); err != nil {
		return err
	}
	if err := grantCoreDnsClusterRole(client, name); err != nil {
		return err
	}
	return nil
}

func grantCoreDnsClusterSA(client clientset.Interface, namespace string) error {
	coreDnsClusterSABytes, err := util.ParseTemplate(host.CoreDnsSA, struct {
		Namespace string
	}{
		Namespace: namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing core-dns sa template: %w", err)
	}
	serviceAccount := &v1.ServiceAccount{}
	if err := yaml.Unmarshal([]byte(coreDnsClusterSABytes), serviceAccount); err != nil {
		return fmt.Errorf("err when decoding core-dns view Clusterrole: %w", err)
	}
	return util.CreateOrUpdateClusterSA(client, serviceAccount, namespace)
}

func grantCoreDnsClusterRoleBinding(client clientset.Interface, namespace string, name string) error {
	coreDnsClusterRoleBindingBytes, err := util.ParseTemplate(host.CoreDnsClusterRoleBinding, struct {
		Name      string
		Namespace string
	}{
		Name:      name,
		Namespace: namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing core-dns role binding template: %w", err)
	}
	viewClusterRoleBinding := &rbacv1.ClusterRoleBinding{}

	if err := yaml.Unmarshal([]byte(coreDnsClusterRoleBindingBytes), viewClusterRoleBinding); err != nil {
		return fmt.Errorf("err when decoding core-dns Clusterrole Binding: %w", err)
	}
	return util.CreateOrUpdateClusterRoleBinding(client, viewClusterRoleBinding)
}

func grantCoreDnsClusterRole(client clientset.Interface, name string) error {
	viewClusterRole := &rbacv1.ClusterRole{}
	coreDnsClusterRoleBytes, err := util.ParseTemplate(host.CoreDnsClusterRole, struct {
		Name string
	}{
		Name: name,
	})
	if err != nil {
		return fmt.Errorf("error when parsing core-dns cluster role template: %w", err)
	}
	if err := yaml.Unmarshal([]byte(coreDnsClusterRoleBytes), viewClusterRole); err != nil {
		return fmt.Errorf("err when decoding core-dns Clusterrole: %w", err)
	}
	return util.CreateOrUpdateClusterRole(client, viewClusterRole)
}
