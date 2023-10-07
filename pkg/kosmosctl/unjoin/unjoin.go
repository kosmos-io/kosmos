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

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
)

const (
	clusterlinkOperator = "clusterlink-operator"
)

var unjoinExample = templates.Examples(i18n.T(`
		# Unjoin cluster from Kosmos control plane in any cluster, e.g:
		kosmosctl unjoin cluster [cluster-name] --cluster-kubeconfig=[member-kubeconfig] --master-kubeconfig=[master-kubeconfig]

		# Unjoin cluster from Kosmos control plane in master cluster, e.g:
		kosmosctl unjoin cluster [cluster-name] --cluster-kubeconfig=[member-kubeconfig]

		# Unjoin knode from Kosmos control plane in any cluster, e.g:
		kosmosctl unjoin knode [knode-name] --master-kubeconfig=[master-kubeconfig]

		# Unjoin knode from Kosmos control plane in master cluster, e.g:
		kosmosctl unjoin knode [knode-name]
`))

type CommandUnJoinOptions struct {
	MasterKubeConfig  string
	ClusterKubeConfig string

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
	err := cmd.MarkFlagRequired("cluster-kubeconfig")
	if err != nil {
		fmt.Printf("kosmosctl unjoin cmd error, MarkFlagRequired failed: %s", err)
	}
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
		_, err := o.DynamicClient.Resource(util.ClusterGVR).Get(context.TODO(), args[1], metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("kosmosctl unjoin validate warning, clsuter is not found: %s", err)
			}
			return fmt.Errorf("kosmosctl unjoin validate error, get cluster failed: %s", err)
		}
	case "knode":
		_, err := o.DynamicClient.Resource(util.KnodeGVR).Get(context.TODO(), args[1], metav1.GetOptions{})
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
		err := o.runCluster(args[1])
		if err != nil {
			return err
		}
	case "knode":
		err := o.runKnode(args[1])
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *CommandUnJoinOptions) runCluster(clusterName string) error {
	klog.Info("Start removing cluster from kosmos control plane...")
	// 1. delete cluster
	for {
		err := o.DynamicClient.Resource(util.ClusterGVR).Namespace("").Delete(context.TODO(), clusterName, metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				break
			}
			return fmt.Errorf("(cluster) kosmosctl unjoin run error, delete cluster failed: %s", err)
		}
		time.Sleep(3 * time.Second)
	}

	// 2. delete operator
	err := o.Client.CoreV1().ServiceAccounts(util.DefaultNamespace).Delete(context.TODO(), clusterlinkOperator, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(operator) kosmosctl unjoin run error, delete serviceaccout failed: %s", err)
	}
	err = o.Client.AppsV1().Deployments(util.DefaultNamespace).Delete(context.TODO(), clusterlinkOperator, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(operator) kosmosctl unjoin run error, delete deployment failed: %s", err)
	}

	// 3. delete secret
	err = o.Client.CoreV1().Secrets(util.DefaultNamespace).Delete(context.TODO(), util.ControlPanelSecretName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(secret) kosmosctl unjoin run error, delete secret failed: %s", err)
	}

	// 4. delete rbac
	err = o.Client.RbacV1().ClusterRoleBindings().Delete(context.TODO(), util.ExternalIPPoolNamePrefix, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(rbac) kosmosctl unjoin run error, delete clusterrolebinding failed: %s", err)
	}
	err = o.Client.RbacV1().ClusterRoles().Delete(context.TODO(), util.ExternalIPPoolNamePrefix, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(rbac) kosmosctl unjoin run error, delete clusterrole failed: %s", err)
	}

	// 5. delete namespace
	err = o.Client.CoreV1().Namespaces().Delete(context.TODO(), util.DefaultNamespace, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(namespace) kosmosctl unjoin run error, delete namespace failed: %s", err)
	}

	klog.Info("Cluster [" + clusterName + "] was removed.")
	return nil
}

func (o *CommandUnJoinOptions) runKnode(knodeName string) error {
	klog.Info("Start removing knode from kosmos control plane...")
	for {
		err := o.DynamicClient.Resource(util.KnodeGVR).Namespace("").Delete(context.TODO(), knodeName, metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				break
			}
			return fmt.Errorf("(knode) kosmosctl unjoin run error, delete knode failed: %s", err)
		}
		time.Sleep(3 * time.Second)
	}

	klog.Info("Knode [" + knodeName + "] was removed.")
	return nil
}
