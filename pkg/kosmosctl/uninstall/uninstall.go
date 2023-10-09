package uninstall

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
)

const (
	clusterlinkNetworkManager = "clusterlink-network-manager"
	clustertreeKnodeManager   = "clustertree-knode-manager"
)

type CommandUninstallOptions struct {
	Namespace string
	Module    string

	Client kubernetes.Interface
}

// NewCmdUninstall Uninstall the Kosmos control plane in a Kubernetes cluster.
func NewCmdUninstall(f ctlutil.Factory) *cobra.Command {
	o := &CommandUninstallOptions{}

	cmd := &cobra.Command{
		Use:                   "uninstall",
		Short:                 i18n.T("Uninstall the Kosmos control plane in a Kubernetes cluster"),
		Long:                  "",
		Example:               "",
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(f))
			ctlutil.CheckErr(o.Validate())
			ctlutil.CheckErr(o.Run())
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&o.Namespace, "namespace", "n", util.DefaultNamespace, "Kosmos namespace.")
	flags.StringVarP(&o.Module, "module", "m", util.DefaultInstallModule, "Kosmos specify the module to uninstall.")

	return cmd
}

func (o *CommandUninstallOptions) Complete(f ctlutil.Factory) error {
	config, err := f.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall complete error, generate rest config failed: %v", err)
	}

	o.Client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall complete error, generate basic client failed: %v", err)
	}

	return nil
}

func (o *CommandUninstallOptions) Validate() error {
	if len(o.Namespace) == 0 {
		return fmt.Errorf("kosmosctl uninstall validate error, namespace must be specified")
	}

	return nil
}

func (o *CommandUninstallOptions) Run() error {
	klog.Info("Kosmos starts uninstalling.")
	switch o.Module {
	case "clusterlink":
		err := o.runClusterlink()
		if err != nil {
			return err
		}
	case "clustertree":
		err := o.runClustertree()
		if err != nil {
			return err
		}
	case "all":
		err := o.runClusterlink()
		if err != nil {
			return err
		}
		err = o.runClustertree()
		if err != nil {
			return err
		}
		err = o.Client.CoreV1().Namespaces().Delete(context.TODO(), o.Namespace, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("kosmosctl uninstall all module run error, namespace options failed: %v", err)
		}
	}

	return nil
}

func (o *CommandUninstallOptions) runClusterlink() error {
	klog.Info("Start uninstalling clusterlink from kosmos control plane...")
	err := o.Client.AppsV1().Deployments(util.DefaultNamespace).Delete(context.TODO(), clusterlinkNetworkManager, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall clusterlink run error, deployment options failed: %v", err)
	}

	err = o.Client.CoreV1().ServiceAccounts(util.DefaultNamespace).Delete(context.TODO(), clusterlinkNetworkManager, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall clusterlink run error, serviceaccount options failed: %v", err)
	}

	klog.Info("Clusterlink was uninstalled.")
	return nil
}

func (o *CommandUninstallOptions) runClustertree() error {
	klog.Info("Start uninstalling clustertree from kosmos control plane...")
	err := o.Client.AppsV1().Deployments(util.DefaultNamespace).Delete(context.TODO(), clustertreeKnodeManager, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall clusterrouter run error, deployment options failed: %v", err)
	}

	err = o.Client.CoreV1().ConfigMaps(util.DefaultNamespace).Delete(context.TODO(), util.HostKubeConfigName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall clusterrouter run error, configmap options failed: %v", err)
	}

	err = o.Client.CoreV1().ServiceAccounts(util.DefaultNamespace).Delete(context.TODO(), clustertreeKnodeManager, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall clusterrouter run error, serviceaccount options failed: %v", err)
	}

	klog.Info("Clustertree was uninstalled.")
	return nil
}
