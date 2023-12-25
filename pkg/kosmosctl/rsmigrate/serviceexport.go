package rsmigrate

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

var exportExample = templates.Examples(i18n.T(`
		# Export service in control plane
		kosmosctl export service foo  -n namespacefoo --kubeconfig=[control plane kubeconfig]
`))

var exportErr string = "kosmosctl export error"

type CommandExportOptions struct {
	*CommandOptions
}

// NewCmdExport export resource to control plane
func NewCmdExport() *cobra.Command {
	o := &CommandExportOptions{CommandOptions: &CommandOptions{SrcLeafClusterOptions: &LeafClusterOptions{}}}

	cmd := &cobra.Command{
		Use:                   "export",
		Short:                 i18n.T("Export resource to control plane data storage center"),
		Long:                  "",
		Example:               exportExample,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(cmd))
			ctlutil.CheckErr(o.Run(cmd, args))
			return nil
		},
	}

	cmd.Flags().StringVarP(&o.MasterKubeConfig, "kubeconfig", "", "", "Absolute path to the master kubeconfig file.")
	cmd.Flags().StringVar(&o.MasterContext, "context", "", "The name of the kubeconfig context.")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "default", "The namespace scope for this CLI request")

	return cmd
}

func (o *CommandExportOptions) Complete(cmd *cobra.Command) error {
	err := o.CommandOptions.Complete(cmd)
	if err != nil {
		return err
	}
	return nil
}

func (o *CommandExportOptions) Run(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("args is null, resource type should be specified")
	}

	switch args[0] {
	case "svc", "services", "service":
		if len(args[1:]) != 1 {
			return fmt.Errorf("%s, exactly one NAME is required, got %d", exportErr, len(args[1:]))
		}

		var err error
		serviceExport := &mcsv1alpha1.ServiceExport{}
		serviceExport.Namespace = o.Namespace
		serviceExport.Kind = "ServiceExport"
		serviceExport.Name = args[1]

		// Create serviceExport, if exists,return error instead of updating it
		_, err = o.MasterKosmosClient.MulticlusterV1alpha1().ServiceExports(o.Namespace).
			Create(context.TODO(), serviceExport, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("%s, create %s %s/%s %s: %s", exportErr, serviceExport.Kind, o.Namespace, args[1], args[0], err)
		}

		fmt.Printf("Create %s %s/%s successfully!\n", serviceExport.Kind, o.Namespace, args[1])
	default:
		return fmt.Errorf("%s, not support export resouece %s", exportErr, args[0])
	}
	return nil
}
