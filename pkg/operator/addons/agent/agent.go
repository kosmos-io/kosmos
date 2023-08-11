package agent

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/operator/addons/option"
	"github.com/kosmos.io/clusterlink/pkg/operator/addons/utils"
	cmdutil "github.com/kosmos.io/clusterlink/pkg/operator/util"
	utils2 "github.com/kosmos.io/clusterlink/pkg/utils"
)

const (
	ResourceName = "clusterlink-agent"
)

type AgentInstaller struct {
}

func New() *AgentInstaller {
	return &AgentInstaller{}
}

// create daemonset
func applyDaemonSet(opt *option.AddonOption) error {
	clusterlinkAgentDaemonSetBytes, err := utils.ParseTemplate(clusterlinkAgentDaemonSet, DaemonSetReplace{
		Namespace:          opt.GetSpecNamespace(),
		Name:               ResourceName,
		ImageRepository:    opt.GetImageRepository(),
		ProxyConfigMapName: utils2.ProxySecretName,
		Version:            opt.Version,
		ClusterName:        opt.GetName(),
	})

	if err != nil {
		return fmt.Errorf("error when parsing clusterlink agent daemonset template :%v", err)
	}

	if clusterlinkAgentDaemonSetBytes == nil {
		return fmt.Errorf("wait klusterlink agent daemonset  timeout")
	}

	clAgentDaemonSet := &appsv1.DaemonSet{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clusterlinkAgentDaemonSetBytes, clAgentDaemonSet); err != nil {
		return fmt.Errorf("decode agent daemonset error: %v", err)
	}

	if err := cmdutil.CreateOrUpdateDaemonSet(opt.KubeClientSet, clAgentDaemonSet); err != nil {
		return fmt.Errorf("create clusterlink agent daemonset error: %v", err)
	}

	// TODO: wati

	return nil
}

func applySecret(opt *option.AddonOption) error {
	if opt.UseProxy {
		return nil
	}
	if err := clientcmdapi.FlattenConfig(opt.ControlPanelKubeConfig); err != nil {
		return err
	}

	// adminCluster := adminConfig.Contexts[adminConfig.CurrentContext].Cluster

	// Copy the cluster from host-cluster to the bootstrap kubeconfig, contains the CA cert and the server URL
	klog.Infof("[bootstrap-token] copying the cluster from admin.conf to the bootstrap kubeconfig")
	// bootstrapConfig := &clientcmdapi.Config{
	// 	Clusters: map[string]*clientcmdapi.Cluster{
	// 		"": adminConfig.Clusters[adminCluster],
	// 	},
	// }

	bootstrapBytes, err := clientcmd.Write(*opt.ControlPanelKubeConfig)
	if err != nil {
		return err
	}

	// Create or update the Secret in the kube-public namespace
	klog.Infof("[bootstrap-token] creating/updating Secret in kube-public namespace")

	return cmdutil.CreateOrUpdateSecret(opt.KubeClientSet, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils2.ProxySecretName,
			Namespace: opt.GetSpecNamespace(),
		},
		StringData: map[string]string{
			bootstrapapi.KubeConfigKey: string(bootstrapBytes),
		},
	})
}

// Install resources related to CR:cluster
func (i *AgentInstaller) Install(opt *option.AddonOption) error {
	if err := applySecret(opt); err != nil {
		return err
	}
	if err := applyDaemonSet(opt); err != nil {
		return err
	}

	klog.Infof("Install clusterlink agent on cluster successfully")
	return nil
}

// Uninstall resources related to CR:cluster
func (i *AgentInstaller) Uninstall(opt *option.AddonOption) error {
	daemonSetClient := opt.KubeClientSet.AppsV1().DaemonSets(opt.GetSpecNamespace())
	if err := daemonSetClient.Delete(context.TODO(), ResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	configMapClient := opt.KubeClientSet.CoreV1().ConfigMaps(opt.GetSpecNamespace())
	if err := configMapClient.Delete(context.TODO(), utils2.ProxySecretName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	klog.Infof("Uninstall clusterlink agent on cluster successfully")
	return nil
}
