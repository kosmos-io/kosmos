package unjoin

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	ClusterlinkOperator = "clusterlink-operator"
)

var unjoinExample = templates.Examples(i18n.T(`
		# unjoin member1-cluster in master control plane
		kosmosctl unjoin member1-cluster --cluster-kubeconfig=[member-kubeconfig] --master-kubeconfig=[master-kubeconfig]

		# unjoin member1-cluster in current master control plane
		kosmosctl unjoin member1-cluster --cluster-kubeconfig=[member-kubeconfig]
`))

type CommandUnJoinOptions struct {
	MasterKubeConfig  string
	ClusterKubeConfig string

	Client        kubernetes.Interface
	DynamicClient *dynamic.DynamicClient
}

// NewCmdUnJoin Delete this in Clusterlink control plane
func NewCmdUnJoin(f ctlutil.Factory) *cobra.Command {
	o := &CommandUnJoinOptions{}

	cmd := &cobra.Command{
		Use:                   "unjoin",
		Short:                 i18n.T("Unjoin this in clusterlink control plane"),
		Long:                  "",
		Example:               unjoinExample,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(f, cmd, args))
			ctlutil.CheckErr(o.Validate())
			ctlutil.CheckErr(o.Run(f, cmd, args))
			return nil
		},
	}

	cmd.Flags().StringVarP(&o.MasterKubeConfig, "master-kubeconfig", "", "", "master-kubeconfig")
	cmd.Flags().StringVarP(&o.ClusterKubeConfig, "cluster-kubeconfig", "", "", "cluster-kubeconfig")
	err := cmd.MarkFlagRequired("cluster-kubeconfig")
	if err != nil {
		fmt.Printf("kosmosctl join cmd error, MarkFlagRequired failed: %s", err)
	}
	return cmd
}
func (o *CommandUnJoinOptions) Complete(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
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

func (o *CommandUnJoinOptions) Validate() error {
	return nil
}

func (o *CommandUnJoinOptions) Run(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
	//delete cluster
	err := o.DynamicClient.Resource(schema.GroupVersionResource{
		Group:    "kosmos.io",
		Version:  "v1alpha1",
		Resource: "clusters",
	}).Namespace("").Delete(context.TODO(), args[0], metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(cluster) kosmosctl unjoin run error, delete cluster failed: %s", err)
	}

	for {
		_, err := o.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    "kosmos.io",
			Version:  "v1alpha1",
			Resource: "clusters",
		}).Namespace("").Get(context.TODO(), args[0], metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				break
			}
			return fmt.Errorf("(cluster) kosmosctl unjoin run error, delete cluster failed: %s", err)
		}
		// Wait 3 second
		time.Sleep(3 * time.Second)
	}

	// delete operator
	err = o.Client.CoreV1().ServiceAccounts(util.DefaultNamespace).Delete(context.TODO(), ClusterlinkOperator, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(operator) kosmosctl unjoin run error, delete serviceaccout failed: %s", err)
	}
	err = o.Client.AppsV1().Deployments(util.DefaultNamespace).Delete(context.TODO(), ClusterlinkOperator, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(operator) kosmosctl unjoin run error, delete deployment failed: %s", err)
	}

	//delete secret
	err = o.Client.CoreV1().Secrets(util.DefaultNamespace).Delete(context.TODO(), utils.ControlPanelSecretName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(secret) kosmosctl unjoin run error, delete secret failed: %s", err)
	}

	//delete namespace
	err = o.Client.CoreV1().Namespaces().Delete(context.TODO(), util.DefaultNamespace, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(namespace) kosmosctl unjoin run error, delete namespace failed: %s", err)
	}

	//delete rbac
	err = o.Client.RbacV1().ClusterRoleBindings().Delete(context.TODO(), utils.ExternalIPPoolNamePrefix, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(rbac) kosmosctl unjoin run error, delete clusterrolebinding failed: %s", err)
	}
	err = o.Client.RbacV1().ClusterRoles().Delete(context.TODO(), utils.ExternalIPPoolNamePrefix, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("(rbac) kosmosctl unjoin run error, delete clusterrole failed: %s", err)
	}

	return nil
}
