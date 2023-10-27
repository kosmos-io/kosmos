package rsmigrate

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

var importExample = templates.Examples(i18n.T(`
		# Import service from control plane to leafcluster
		kosmosctl import service foo -n namespacefoo --kubecnfig=[control plane kubeconfig] --to-leafcluster leafclusterfoo
`))

var importErr string = "kosmosctl import error"

type CommandImportOptions struct {
	*CommandOptions
	DstLeafClusterOptions *LeafClusterOptions
}

// NewCmdImport import resource
func NewCmdImport(f ctlutil.Factory) *cobra.Command {
	o := &CommandImportOptions{
		CommandOptions:        &CommandOptions{SrcLeafClusterOptions: &LeafClusterOptions{}},
		DstLeafClusterOptions: &LeafClusterOptions{},
	}

	cmd := &cobra.Command{
		Use:                   "import",
		Short:                 i18n.T("Import resource to leafCluster"),
		Long:                  "",
		Example:               importExample,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(f, cmd))
			ctlutil.CheckErr(o.Validate(cmd))
			ctlutil.CheckErr(o.Run(f, cmd, args))
			return nil
		},
	}
	cmd.Flags().StringVarP(&o.MasterKubeConfig, "kubeconfig", "", "", "Absolute path to the master kubeconfig file.")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "default", "The namespace scope for this CLI request")
	cmd.Flags().StringVar(&o.DstLeafClusterOptions.LeafClusterName, "to-leafcluster", "", "Import resource to this destination leafcluster")

	return cmd
}

func (o *CommandImportOptions) Complete(f ctlutil.Factory, cmd *cobra.Command) error {
	err := o.CommandOptions.Complete(f, cmd)
	if err != nil {
		return err
	}

	// get dst leafCluster options
	if cmd.Flags().Changed("to-leafcluster") {
		err := completeLeafClusterOptions(o.DstLeafClusterOptions, o.MasterKosmosClient)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *CommandImportOptions) Validate(cmd *cobra.Command) error {
	err := o.CommandOptions.Validate(cmd)
	if err != nil {
		return fmt.Errorf("%s, valid args error: %s", importErr, err)
	}

	if !cmd.Flags().Changed("to-leafcluster") {
		return fmt.Errorf("%s, required flag(s) 'to-leafcluster' not set", importErr)
	}
	return nil
}

func (o *CommandImportOptions) Run(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("args is null, resource should be specified")
	}

	switch args[0] {
	case "svc", "services", "service":
		if len(args[1:]) != 1 {
			return fmt.Errorf("%s, exactly one NAME is required, got %d", importErr, len(args[1:]))
		}

		var srcService *v1.Service
		var err error
		if o.SrcLeafClusterOptions.LeafClusterName != "" {
			srcService, err = o.SrcLeafClusterOptions.LeafClusterNativeClient.CoreV1().Services(o.Namespace).Get(context.TODO(), args[1], metav1.GetOptions{})
		} else {
			srcService, err = o.MasterClient.CoreV1().Services(o.Namespace).Get(context.TODO(), args[1], metav1.GetOptions{})
		}
		if err != nil {
			return fmt.Errorf("%s, get source service %s/%s error: %s", importErr, o.Namespace, args[1], err)
		}

		serviceImport := &mcsv1alpha1.ServiceImport{}
		serviceImport.Kind = "ServiceImport"
		serviceImport.Namespace = o.Namespace
		serviceImport.Name = args[1]
		serviceImport.Spec.Type = "ClusterSetIP"
		if srcService.Spec.ClusterIP == "None" || len(srcService.Spec.ClusterIP) == 0 {
			serviceImport.Spec.Type = "Headless"
		}

		serviceImport.Spec.Ports = make([]mcsv1alpha1.ServicePort, len(srcService.Spec.Ports))
		for portIndex, svcPort := range srcService.Spec.Ports {
			serviceImport.Spec.Ports[portIndex] = mcsv1alpha1.ServicePort{
				Name:        svcPort.Name,
				Protocol:    svcPort.Protocol,
				AppProtocol: svcPort.AppProtocol,
				Port:        svcPort.Port,
			}
		}

		// Create serviceImport, if exists,return error instead of updating it
		if len(o.DstLeafClusterOptions.LeafClusterName) != 0 {
			_, err = o.DstLeafClusterOptions.LeafClusterKosmosClient.MulticlusterV1alpha1().ServiceImports(o.Namespace).
				Create(context.TODO(), serviceImport, metav1.CreateOptions{})
		} else {
			_, err = o.MasterKosmosClient.MulticlusterV1alpha1().ServiceImports(o.Namespace).
				Create(context.TODO(), serviceImport, metav1.CreateOptions{})
		}
		if err != nil {
			return fmt.Errorf("%s, create %s %s/%s %s: %s", importErr, serviceImport.Kind, o.Namespace, args[1], args[0], err)
		}

		fmt.Printf("Create %s %s/%s successfully!\n", serviceImport.Kind, o.Namespace, args[1])
	default:
		return fmt.Errorf("%s, not support import resouece %s", importErr, args[0])
	}

	return nil
}
