package rbac

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/treeoperator/util"
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
	virtualClusterResourceClusterSABytes, err := util.ParseTemplate(VirtualSchedulerSA, struct {
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
	return createOrUpdateClusterSA(client, serviceAccount, namespace)
}

func grantVirtualClusterResourceClusterRoleBinding(client clientset.Interface, namespace string) error {
	virtualClusterResourceClusterRoleBindingBytes, err := util.ParseTemplate(VirtualSchedulerRoleBinding, struct {
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
	return createOrUpdateClusterRoleBinding(client, viewClusterRoleBinding)
}

func grantVirtualClusterResourceClusterRole(client clientset.Interface) error {
	viewClusterrole := &rbacv1.ClusterRole{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(VirtualSchedulerRole), viewClusterrole); err != nil {
		return fmt.Errorf("err when decoding virtualCluster scheduler  Clusterrole: %w", err)
	}
	return createOrUpdateClusterRole(client, viewClusterrole)
}

func createOrUpdateClusterSA(client clientset.Interface, serviceAccount *v1.ServiceAccount, namespace string) error {
	_, err := client.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		older, err := client.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), serviceAccount.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		serviceAccount.ResourceVersion = older.ResourceVersion
		_, err = client.CoreV1().ServiceAccounts(namespace).Update(context.TODO(), serviceAccount, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(4).InfoS("Successfully created or updated serviceAccount", "serviceAccount", serviceAccount.GetName)
	return nil
}

func createOrUpdateClusterRole(client clientset.Interface, clusterrole *rbacv1.ClusterRole) error {
	_, err := client.RbacV1().ClusterRoles().Create(context.TODO(), clusterrole, metav1.CreateOptions{})

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		older, err := client.RbacV1().ClusterRoles().Get(context.TODO(), clusterrole.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		clusterrole.ResourceVersion = older.ResourceVersion
		_, err = client.RbacV1().ClusterRoles().Update(context.TODO(), clusterrole, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(4).InfoS("Successfully created or updated clusterrole", "clusterrole", clusterrole.GetName)
	return nil
}

func createOrUpdateClusterRoleBinding(client clientset.Interface, clusterroleBinding *rbacv1.ClusterRoleBinding) error {
	_, err := client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterroleBinding, metav1.CreateOptions{})

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		older, err := client.RbacV1().ClusterRoles().Get(context.TODO(), clusterroleBinding.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		clusterroleBinding.ResourceVersion = older.ResourceVersion
		_, err = client.RbacV1().ClusterRoleBindings().Update(context.TODO(), clusterroleBinding, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(4).InfoS("Successfully created or updated clusterrolebinding", "clusterrolebinding", clusterroleBinding.GetName)
	return nil
}
