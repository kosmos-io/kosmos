package unjoin

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

var unjoinExample = templates.Examples(i18n.T(`
		# Unjoin cluster from Kosmos control plane in any cluster, e.g:
		kosmosctl unjoin cluster --cluster-name=[cluster-name] --cluster-kubeconfig=[member-kubeconfig] --master-kubeconfig=[master-kubeconfig]

		# Unjoin cluster from Kosmos control plane in master cluster, e.g:
		kosmosctl unjoin cluster --cluster-name=[cluster-name] --cluster-kubeconfig=[member-kubeconfig]

		# Unjoin knode from Kosmos control plane in any cluster, e.g:
		kosmosctl unjoin knode --knode-name=[knode-name] --master-kubeconfig=[master-kubeconfig]

		# Unjoin knode from Kosmos control plane in master cluster, e.g:
		kosmosctl unjoin knode --knode-name=[knode-name]
`))

type CommandUnJoinOptions struct {
	MasterKubeConfig  string
	ClusterKubeConfig string

	ClusterName string

	KnodeName string

	Client        kubernetes.Interface
	DynamicClient *dynamic.DynamicClient
}

// NewCmdUnJoin Delete resource in Kosmos control plane.
func NewCmdUnJoin(f ctlutil.Factory) *cobra.Command {
	o := &CommandUnJoinOptions{}

	cmd := &cobra.Command{
		Use:                   "unjoin",
		Short:                 i18n.T("Unjoin resource in kosmos control plane"),
		Long:                  "",
		Example:               unjoinExample,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(f))
			ctlutil.CheckErr(o.Validate(args))
			ctlutil.CheckErr(o.Run(args))
			return nil
		},
	}

	cmd.Flags().StringVarP(&o.MasterKubeConfig, "master-kubeconfig", "", "", "Absolute path to the master kubeconfig file.")
	cmd.Flags().StringVarP(&o.ClusterKubeConfig, "cluster-kubeconfig", "", "", "Absolute path to the cluster kubeconfig file.")
	cmd.Flags().StringVar(&o.ClusterName, "cluster-name", "", "Specify the name of the member cluster to unjoin.")
	cmd.Flags().StringVar(&o.KnodeName, "knode-name", "", "Specify the name of the knode to unjoin.")

	return cmd
}
func (o *CommandUnJoinOptions) Complete(f ctlutil.Factory) error {
	var masterConfig *restclient.Config
	var err error

	if o.MasterKubeConfig != "" {
		masterConfig, err = clientcmd.BuildConfigFromFlags("", o.MasterKubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl unjoin complete error, generate masterConfig failed: %s", err)
		}
	} else {
		masterConfig, err = f.ToRESTConfig()
		if err != nil {
			return fmt.Errorf("kosmosctl unjoin complete error, get current masterConfig failed: %s", err)
		}
	}

	clusterConfig, err := clientcmd.BuildConfigFromFlags("", o.ClusterKubeConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl unjoin complete error, generate memberConfig failed: %s", err)
	}

	o.Client, err = kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate basic client failed: %v", err)
	}

	o.DynamicClient, err = dynamic.NewForConfig(masterConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl unjoin complete error, generate dynamic client failed: %s", err)
	}

	return nil
}

func (o *CommandUnJoinOptions) Validate(args []string) error {
	switch args[0] {
	case "cluster":
		_, err := o.DynamicClient.Resource(util.ClusterGVR).Get(context.TODO(), o.ClusterName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("kosmosctl unjoin validate warning, clsuter is not found: %s", err)
			}
			return fmt.Errorf("kosmosctl unjoin validate error, get cluster failed: %s", err)
		}
	case "knode":
		_, err := o.DynamicClient.Resource(util.KnodeGVR).Get(context.TODO(), o.KnodeName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("kosmosctl unjoin validate warning, knode is not found: %s", err)
			}
			return fmt.Errorf("kosmosctl unjoin validate error, get knode failed: %s", err)
		}
	}

	return nil
}

func (o *CommandUnJoinOptions) Run(args []string) error {
	switch args[0] {
	case "cluster":
		err := o.runCluster()
		if err != nil {
			return err
		}
	case "knode":
		err := o.runKnode()
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *CommandUnJoinOptions) runCluster() error {
	klog.Info("Start removing cluster from kosmos control plane...")
	// 1. delete cluster
	for {
		err := o.DynamicClient.Resource(util.ClusterGVR).Namespace("").Delete(context.TODO(), o.ClusterName, metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				break
			}
			return fmt.Errorf("(cluster) kosmosctl unjoin run error, delete cluster failed: %s", err)
		}
		time.Sleep(3 * time.Second)
	}

	// 2. delete operator
	clusterlinkOperatorDeployment, err := util.GenerateDeployment(manifest.ClusterlinkOperatorDeployment, nil)
	if err != nil {
		return err
	}
	err = o.Client.AppsV1().Deployments(utils.DefaultNamespace).Delete(context.TODO(), clusterlinkOperatorDeployment.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(operator) kosmosctl unjoin run error, delete deployment failed: %s", err)
	}

	// 3. delete secret
	err = o.Client.CoreV1().Secrets(utils.DefaultNamespace).Delete(context.TODO(), utils.ControlPanelSecretName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(secret) kosmosctl unjoin run error, delete secret failed: %s", err)
	}

	// 4. delete rbac
	err = o.Client.RbacV1().ClusterRoleBindings().Delete(context.TODO(), utils.ExternalIPPoolNamePrefix, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(rbac) kosmosctl unjoin run error, delete clusterrolebinding failed: %s", err)
	}
	err = o.Client.RbacV1().ClusterRoles().Delete(context.TODO(), utils.ExternalIPPoolNamePrefix, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(rbac) kosmosctl unjoin run error, delete clusterrole failed: %s", err)
	}
	clusterlinkOperatorServiceAccount, err := util.GenerateServiceAccount(manifest.ClusterlinkOperatorServiceAccount, nil)
	if err != nil {
		return err
	}
	err = o.Client.CoreV1().ServiceAccounts(utils.DefaultNamespace).Delete(context.TODO(), clusterlinkOperatorServiceAccount.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(operator) kosmosctl unjoin run error, delete serviceaccout failed: %s", err)
	}

	// 5. If cluster is not the master, delete namespace
	clusterlinkNetworkManagerDeployment, err := util.GenerateDeployment(manifest.ClusterlinkNetworkManagerDeployment, nil)
	if err != nil {
		return err
	}
	_, err = o.Client.AppsV1().Deployments(utils.DefaultNamespace).Get(context.TODO(), clusterlinkNetworkManagerDeployment.Name, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		err = o.Client.CoreV1().Namespaces().Delete(context.TODO(), utils.DefaultNamespace, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("(namespace) kosmosctl unjoin run error, delete namespace failed: %s", err)
		}
	}

	klog.Info("Cluster [" + o.ClusterName + "] was removed.")
	return nil
}

func (o *CommandUnJoinOptions) runKnode() error {
	klog.Info("Start removing knode from kosmos control plane...")
	for {
		err := o.DynamicClient.Resource(util.KnodeGVR).Namespace("").Delete(context.TODO(), o.KnodeName, metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				break
			}
			return fmt.Errorf("(knode) kosmosctl unjoin run error, delete knode failed: %s", err)
		}
		time.Sleep(3 * time.Second)
	}

	klog.Info("Knode [" + o.KnodeName + "] was removed.")
	return nil
}
