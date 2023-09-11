package uninstall

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	extensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	ctldelete "k8s.io/kubectl/pkg/cmd/delete"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
)

const (
	clusterlinkSystem = "clusterlink-system"
)

type CommandUninstallOptions struct {
	Namespace string

	CoreClient          *coreclient.CoreV1Client
	ExtensionsClientSet extensionsclientset.Interface

	DelDeploymentOptions     *ctldelete.DeleteOptions
	DelServiceAccountOptions *ctldelete.DeleteOptions
}

// NewCmdUninstall Uninstall the Kosmos control plane in a Kubernetes cluster.
func NewCmdUninstall(f ctlutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o, err := NewCommandUninstallOptions(streams)
	ctlutil.CheckErr(err)

	cmd := &cobra.Command{
		Use:                   "uninstall",
		Short:                 i18n.T("Uninstall the Kosmos control plane in a Kubernetes cluster"),
		Long:                  "",
		Example:               "",
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(f, cmd, args))
			ctlutil.CheckErr(o.Validate())
			ctlutil.CheckErr(o.Run(f))
			return nil
		},
	}
	ctlutil.AddDryRunFlag(cmd)

	flags := cmd.Flags()
	flags.StringVarP(&o.Namespace, "namespace", "n", clusterlinkSystem, "Kosmos namespace.")

	return cmd
}

// NewCommandUninstallOptions returns a CommandUninstallOptions.
func NewCommandUninstallOptions(streams genericclioptions.IOStreams) (*CommandUninstallOptions, error) {
	delFlags := ctldelete.NewDeleteFlags("Contains resources for Kosmos to delete.")
	delOptions, err := delFlags.ToOptions(nil, streams)
	if err != nil {
		return nil, fmt.Errorf("kosmosctl uninstall error, generate uninstall options failed: %s", err)
	}

	return &CommandUninstallOptions{
		DelDeploymentOptions:     delOptions,
		DelServiceAccountOptions: delOptions,
	}, nil
}

func (o *CommandUninstallOptions) Complete(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
	config, err := f.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall complete error, generate rest config failed: %v", err)
	}

	coreClient, err := coreclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall complete error, create core client failed: %v", err)
	}
	o.CoreClient = coreClient

	o.DelDeploymentOptions.FieldSelector = "metadata.namespace=" + o.Namespace
	err = o.DelDeploymentOptions.Complete(f, []string{"deploy"}, cmd)
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall complete error, deployment options failed: %v", err)
	}

	o.DelServiceAccountOptions.FieldSelector = "metadata.name!=default,metadata.namespace=" + o.Namespace
	err = o.DelServiceAccountOptions.Complete(f, []string{"sa"}, cmd)
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall complete error, serviceaccount options failed: %v", err)
	}

	return nil
}

func (o *CommandUninstallOptions) Validate() error {
	if len(o.Namespace) == 0 {
		return fmt.Errorf("kosmosctl uninstall validate error, namespace must be specified")
	}

	err := o.DelDeploymentOptions.Validate()
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall validate error, deployment options failed: %v", err)
	}

	err = o.DelServiceAccountOptions.Validate()
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall validate error, serviceaccount options failed: %v", err)
	}

	return nil
}

func (o *CommandUninstallOptions) Run(f ctlutil.Factory) error {
	err := o.DelDeploymentOptions.RunDelete(f)
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall run error, deployment options failed: %v", err)
	}

	err = o.DelServiceAccountOptions.RunDelete(f)
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall run error, serviceaccount options failed: %v", err)
	}

	err = o.CoreClient.Namespaces().Delete(context.TODO(), o.Namespace, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl uninstall run error, deployment options failed: %v", err)
		}
	}

	return nil
}
