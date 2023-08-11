package ctlmaster

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	apiclient "github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/util/apiclient"
	"github.com/kosmos.io/clusterlink/pkg/operator/addons/utils"
)

var serviceaccountTemplateNameMap = map[string]string{
	"clusterlink-operator":           clusterlinkOperatorServiceAccount,
	"clusterlink-controller-manager": clusterlinkControllerServiceAccount,
}

func (i *CommandInitOption) initClusterlinkClusterRole() error {

	klog.Info("Create Clusterlink ClusterRole")
	clusterlinkClusterRoleBytes, err := utils.ParseTemplate(clusterlinkClusterRole,
		RBACStuctNull{})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink ClusterRole template :%v", err)
	} else if clusterlinkClusterRoleBytes == nil {
		return fmt.Errorf("ClusterRoles template get nil")
	}

	// get ClusterRole struct
	clusterlinkClusterRoleStruct := &rbacv1.ClusterRole{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(),
		clusterlinkClusterRoleBytes, clusterlinkClusterRoleStruct); err != nil {

		return fmt.Errorf("decode clusterlinkClusterRoleBytes error : %v ", err)
	}

	// create or update
	if _, err := i.KubeClientSet.RbacV1().ClusterRoles().Create(context.TODO(), clusterlinkClusterRoleStruct,
		metav1.CreateOptions{}); err != nil {

		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create ClusterRole error : %v ", err)
		} else if apierrors.IsAlreadyExists(err) {
			klog.Info("Resource ClusterRole clusterlink already exists, Update it")
			if _, err := i.KubeClientSet.RbacV1().ClusterRoles().Update(context.TODO(), clusterlinkClusterRoleStruct,
				metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("update ClusterRole error : %v ", err)
			}
		}
	}

	return nil
}

func (i *CommandInitOption) retriveClusterLinkSA() ([]KubeResourceInfo, error) {

	SaInK8sList := []KubeResourceInfo{}
	saDeployedByCL := []string{}
	for k := range serviceaccountTemplateNameMap {
		saDeployedByCL = append(saDeployedByCL, k)
	}
	serviceaccounts, err := i.KubeClientSet.CoreV1().ServiceAccounts("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("can't get serviceaccounts from kubernetes")
		return nil, err
	}
	for _, sa := range serviceaccounts.Items {
		if apiclient.ContainsInSlice(saDeployedByCL, sa.Name) {
			SaInK8sList = append(SaInK8sList, KubeResourceInfo{
				Name:           sa.Name,
				Namespace:      sa.Namespace,
				ResourceClient: i.KubeClientSet.CoreV1().ServiceAccounts(sa.Namespace),
				Type:           "serviceaccount",
			})
		}
	}
	return SaInK8sList, nil
}

func (i *CommandInitOption) initClusterlinkClusterRoleBinding() error {

	klog.Info("Create Clusterlink ClusterRoleBing")
	clusterlinkClusterRoleBindingBytes, err := utils.ParseTemplate(clusterlinkClusterRoleBinding,
		ClusterRoleBindingReplace{
			Namespace: i.Namespace,
		})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink ClusterRoleBinding template :%v", err)
	} else if clusterlinkClusterRoleBindingBytes == nil {
		return fmt.Errorf("ClusterRolesBinding template get nil")
	}

	// get ClusterRoleBing struct
	clusterlinkClusterRoleBindingStruct := &rbacv1.ClusterRoleBinding{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(),
		clusterlinkClusterRoleBindingBytes, clusterlinkClusterRoleBindingStruct); err != nil {
		return fmt.Errorf("decode clusterlink ClusterRoleBindingBytes error : %v ", err)
	}

	// create or update
	if _, err := i.KubeClientSet.RbacV1().ClusterRoleBindings().Create(context.TODO(),
		clusterlinkClusterRoleBindingStruct, metav1.CreateOptions{}); err != nil {

		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create ClusterRoleBinding error : %v ", err)
		} else {
			klog.Info("Resource ClusterRoleBinding clusterlink already exists, Update it")
			if _, err := i.KubeClientSet.RbacV1().ClusterRoleBindings().Update(context.TODO(),
				clusterlinkClusterRoleBindingStruct, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("update ClusterRoleBinding error : %v ", err)
			}
		}
	}

	return nil
}

func (i *CommandInitOption) initClusterlinkServiceAccount() error {
	for _, v := range serviceaccountTemplateNameMap {
		if err := i.deployClusterlinkServiceAccount(v); err != nil {
			return err
		}
	}
	return nil
}

func (i *CommandInitOption) deployClusterlinkServiceAccount(satemplate string) error {

	klog.Info("Create Clusterlink ServiceAccount")
	clusterlinkServiceAccountBytes, err := utils.ParseTemplate(satemplate,
		ServiceAccountReplace{
			Namespace: i.Namespace,
		})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink ServiceAccount template :%v", err)
	} else if clusterlinkServiceAccountBytes == nil {
		return fmt.Errorf("ServiceAccount template get nil")
	}

	// get ServiceAccounts struct
	clusterlinkServiceAccountStruct := &corev1.ServiceAccount{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(),
		clusterlinkServiceAccountBytes, clusterlinkServiceAccountStruct); err != nil {
		return fmt.Errorf("decode clusterlink ServiceAccount Bytes error : %w ", err)
	}

	// create or update
	if _, err := i.KubeClientSet.CoreV1().ServiceAccounts(i.Namespace).Create(context.TODO(),
		clusterlinkServiceAccountStruct, metav1.CreateOptions{}); err != nil {

		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create ServiceAccount %s error : %w ", clusterlinkServiceAccountStruct.Name, err)
		} else {
			klog.Infof("Resource ServiceAccount %s already exists, Update it", clusterlinkServiceAccountStruct.Name)
			if _, err := i.KubeClientSet.CoreV1().ServiceAccounts(i.Namespace).Update(context.TODO(),
				clusterlinkServiceAccountStruct, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("update ServiceAccount %s error : %v ", clusterlinkServiceAccountStruct.Name, err)
			}
		}
	}

	return nil
}

func (i *CommandInitOption) initClusterlinkRBAC() error {

	klog.Info("Create Clusterlink RBAC")

	err := i.initClusterlinkClusterRole()
	if err != nil {
		return err
	}

	err = i.initClusterlinkClusterRoleBinding()
	if err != nil {
		return err
	}

	err = i.initClusterlinkServiceAccount()
	if err != nil {
		return err
	}

	return nil
}
