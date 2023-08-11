package proxy

import (
	"context"
	"errors"
	"fmt"
	utils2 "github.com/kosmos.io/clusterlink/pkg/utils"
	"k8s.io/client-go/tools/clientcmd"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/operator/addons/option"
	"github.com/kosmos.io/clusterlink/pkg/operator/addons/utils"
	cmdutil "github.com/kosmos.io/clusterlink/pkg/operator/util"
)

type ProxyInstaller struct {
}

func New() *ProxyInstaller {
	return &ProxyInstaller{}
}

const (
	ResourceName = "clusterlink-proxy"
)

var (
	clusterClientSet *kubernetes.Clientset
)

func applySecret(opt *option.AddonOption) error {
	if opt.ControlPanelKubeConfig == nil {
		return errors.New("ControlPanelKubeConfig must not nil")
	}

	c := opt.ControlPanelKubeConfig.DeepCopy()
	url := fmt.Sprintf("https://%s:443/apis/clusterlink.io/v1alpha1/proxying", ResourceName)
	klog.Infof("proxy access url is %s", url)
	for i := range c.Clusters {
		c.Clusters[i].Server = url
	}
	b, err := clientcmd.Write(*c)
	if err != nil {
		klog.Errorf("write ControlPanelKubeConfig to byte err: %v")
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils2.ProxySecretName,
			Namespace: opt.GetSpecNamespace(),
		},
		Data: map[string][]byte{
			"kubeconfig": b,
		},
	}

	if err := cmdutil.CreateOrUpdateSecret(opt.KubeClientSet, secret); err != nil {
		return fmt.Errorf("create clusterlink agent secret error: %v", err)
	}

	return nil

}

func applyService(opt *option.AddonOption) error {
	proxyServiceBytes, err := utils.ParseTemplate(clusterlinkProxyService, ServiceReplace{
		Namespace: opt.GetSpecNamespace(),
		Name:      ResourceName,
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink controller-manager serviceaccount template :%v", err)
	}

	proxyService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), proxyServiceBytes, proxyService); err != nil {
		return fmt.Errorf("decode controller-proxy service error: %v", err)
	}

	if err := cmdutil.CreateOrUpdateService(opt.KubeClientSet, proxyService); err != nil {
		return fmt.Errorf("create clusterlink-proxy service error: %v", err)
	}

	// TODO: wati

	return nil
}

func applyDeployment(opt *option.AddonOption) error {
	proxyDeploymentBytes, err := utils.ParseTemplate(clusterlinkProxyDeployment, DeploymentReplace{
		Namespace:              opt.GetSpecNamespace(),
		Name:                   ResourceName,
		ControlPanelSecretName: utils2.ControlPanelSecretName,
		ImageRepository:        opt.GetImageRepository(),
		Version:                opt.Version,
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink-proxy deployment template :%v", err)
	}

	proxyDeployment := &appsv1.Deployment{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), proxyDeploymentBytes, proxyDeployment); err != nil {
		return fmt.Errorf("decode clusterlink-proxy deployment error: %v", err)
	}

	if err := cmdutil.CreateOrUpdateDeployment(opt.KubeClientSet, proxyDeployment); err != nil {
		return fmt.Errorf("create controller-proxy deployment error: %v", err)
	}

	// TODO: wati

	return nil
}

// Install resources related to CR:cluster
func (i *ProxyInstaller) Install(opt *option.AddonOption) error {
	if !opt.UseProxy {
		return nil
	}
	klog.Infof("deploying proxy...")

	if err := applySecret(opt); err != nil {
		return err
	}

	if err := applyDeployment(opt); err != nil {
		return err
	}

	if err := applyService(opt); err != nil {
		return err
	}

	return nil
}

// Uninstall resources related to CR:cluster
func (i *ProxyInstaller) Uninstall(opt *option.AddonOption) error {
	deploymentClient := opt.KubeClientSet.AppsV1().Deployments(opt.GetSpecNamespace())
	if err := deploymentClient.Delete(context.TODO(), ResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	serviceClient := opt.KubeClientSet.CoreV1().Services(opt.GetSpecNamespace())
	if err := serviceClient.Delete(context.TODO(), ResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	klog.Infof("Uninstall clusterlink service on cluster successfully")
	return nil
}
