package get

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	ctlget "k8s.io/kubectl/pkg/cmd/get"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"

	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

const (
	ClustersGroupResource     = "clusters.kosmos.io"
	ClusterNodesGroupResource = "clusternodes.kosmos.io"
)

type CommandGetOptions struct {
	Cluster     string
	ClusterNode string

	GetOptions *ctlget.GetOptions

	clusterLinkClient *versioned.Clientset
}

// NewCmdGet Display resources from the Clusterlink control plane.
func NewCmdGet(f ctlutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCommandGetOptions(streams)

	cmd := &cobra.Command{
		Use:                   fmt.Sprintf("get [(-o|--output=)%s] (TYPE[.VERSION][.GROUP] [NAME | -l label] | TYPE[.VERSION][.GROUP]/NAME ...) [flags]", strings.Join(o.GetOptions.PrintFlags.AllowedFormats(), "|")),
		Short:                 i18n.T("Display resources from the Clusterlink control plane"),
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

	return cmd
}

// NewCommandGetOptions returns a CommandGetOptions.
func NewCommandGetOptions(streams genericclioptions.IOStreams) *CommandGetOptions {
	getOptions := ctlget.NewGetOptions("linkctl", streams)
	return &CommandGetOptions{
		GetOptions: getOptions,
	}
}

func (o *CommandGetOptions) Complete(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
	config, err := f.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("linkctl get complete error, generate rest config failed: %s", err)
	}

	o.clusterLinkClient, err = versioned.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("linkctl get complete error, create clusterlink client failed: %s", err)
	}

	err = o.GetOptions.Complete(f, cmd, args)
	if err != nil {
		return fmt.Errorf("linkctl get complete error, options failed: %s", err)
	}

	return nil
}

func (o *CommandGetOptions) Validate() error {
	err := o.GetOptions.Validate()
	if err != nil {
		return fmt.Errorf("linkctl get validate error, options failed: %s", err)
	}

	return nil
}

func (o *CommandGetOptions) Run(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
	for i := range args {
		switch args[i] {
		case "cluster", "clusters":
			args[i] = ClustersGroupResource
		case "clusternode", "clusternodes":
			args[i] = ClusterNodesGroupResource
		}
	}

	err := o.GetOptions.Run(f, cmd, args)
	if err != nil {
		return fmt.Errorf("linkctl get run error, options failed: %s", err)
	}

	return nil
}
