package get

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	ctlget "k8s.io/kubectl/pkg/cmd/get"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
)

const (
	ClustersGroupResource     = "clusters.kosmos.io"
	ClusterNodesGroupResource = "clusternodes.kosmos.io"
	KnodesGroupResource       = "knodes.kosmos.io"
)

type CommandGetOptions struct {
	Cluster     string
	ClusterNode string

	Namespace string

	GetOptions *ctlget.GetOptions
}

// NewCmdGet Display resources from the Kosmos control plane.
func NewCmdGet(f ctlutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCommandGetOptions(streams)

	cmd := &cobra.Command{
		Use:                   fmt.Sprintf("get [(-o|--output=)%s] (TYPE[.VERSION][.GROUP] [NAME | -l label] | TYPE[.VERSION][.GROUP]/NAME ...) [flags]", strings.Join(o.GetOptions.PrintFlags.AllowedFormats(), "|")),
		Short:                 i18n.T("Display resources from the Kosmos control plane"),
		Long:                  "",
		Example:               "",
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(f, cmd, args))
			ctlutil.CheckErr(o.Validate())
			ctlutil.CheckErr(o.Run(f, cmd, args))
			return nil
		},
	}

	o.GetOptions.PrintFlags.AddFlags(cmd)
	flags := cmd.Flags()
	flags.StringVarP(&o.Namespace, "namespace", "n", util.DefaultNamespace, "If present, the namespace scope for this CLI request.")

	return cmd
}

// NewCommandGetOptions returns a CommandGetOptions.
func NewCommandGetOptions(streams genericclioptions.IOStreams) *CommandGetOptions {
	getOptions := ctlget.NewGetOptions("kosmosctl", streams)
	return &CommandGetOptions{
		GetOptions: getOptions,
	}
}

func (o *CommandGetOptions) Complete(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
	err := o.GetOptions.Complete(f, cmd, args)
	if err != nil {
		return fmt.Errorf("kosmosctl get complete error, options failed: %s", err)
	}

	o.GetOptions.Namespace = o.Namespace

	return nil
}

func (o *CommandGetOptions) Validate() error {
	err := o.GetOptions.Validate()
	if err != nil {
		return fmt.Errorf("kosmosctl get validate error, options failed: %s", err)
	}

	return nil
}

func (o *CommandGetOptions) Run(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "cluster", "clusters":
		args[0] = ClustersGroupResource
	case "clusternode", "clusternodes":
		args[0] = ClusterNodesGroupResource
	case "knode", "knodes":
		args[0] = KnodesGroupResource
	}

	err := o.GetOptions.Run(f, cmd, args)
	if err != nil {
		return fmt.Errorf("kosmosctl get run error, options failed: %s", err)
	}

	return nil
}
