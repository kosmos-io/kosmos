package util

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func CreateOrUpdateDeployment(client clientset.Interface, deployment *appsv1.Deployment) error {
	_, err := client.AppsV1().Deployments(deployment.GetNamespace()).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		_, err := client.AppsV1().Deployments(deployment.GetNamespace()).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated deployment", "deployment", deployment.GetName())
	return nil
}

func CreateOrUpdateConfigMap(client clientset.Interface, configMap *v1.ConfigMap) error {
	_, err := client.CoreV1().ConfigMaps(configMap.GetNamespace()).Create(context.TODO(), configMap, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		_, err := client.CoreV1().ConfigMaps(configMap.GetNamespace()).Update(context.TODO(), configMap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated configMap", "configMap", configMap.GetName())
	return nil
}

func CreateOrUpdateStatefulSet(client clientset.Interface, statefulSet *appsv1.StatefulSet) error {
	_, err := client.AppsV1().StatefulSets(statefulSet.GetNamespace()).Create(context.TODO(), statefulSet, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		older, err := client.AppsV1().StatefulSets(statefulSet.GetNamespace()).Get(context.TODO(), statefulSet.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		statefulSet.ResourceVersion = older.ResourceVersion
		_, err = client.AppsV1().StatefulSets(statefulSet.GetNamespace()).Update(context.TODO(), statefulSet, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated statefulset", "statefulset", statefulSet.GetName)
	return nil
}

func CreateOrUpdateClusterSA(client clientset.Interface, serviceAccount *v1.ServiceAccount, namespace string) error {
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

func CreateOrUpdateClusterRole(client clientset.Interface, clusterrole *rbacv1.ClusterRole) error {
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

func CreateOrUpdateClusterRoleBinding(client clientset.Interface, clusterroleBinding *rbacv1.ClusterRoleBinding) error {
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
