package app

import (
	"flag"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	apiserverflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/floaterclient"
	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/initmaster"
	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/membercurd"
	"github.com/kosmos.io/clusterlink/pkg/version"
)

var (
	rootCmdShort = "%s controls a Kubernetes cluster netWork "
	rootCmdLong  = "Welcome to Cluster Link, %s can help you to link your Kubernetes clusters"
)

// NewLinkCtlCommand creates the `linkctl` command.
func NewLinkCtlCommand(cmdUse, parentCommand string) *cobra.Command {
	// Parent command to which all sub-commands are added.
	rootCmd := &cobra.Command{
		Use:   cmdUse,
		Short: fmt.Sprintf(rootCmdShort, parentCommand),
		Long:  fmt.Sprintf(rootCmdLong, parentCommand),

		RunE: runHelp,
	}

	// Init log flags
	klog.InitFlags(flag.CommandLine)

	pflag.CommandLine.SetNormalizeFunc(apiserverflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	rootCmd.PersistentFlags().AddFlagSet(pflag.CommandLine)

	if err := flag.CommandLine.Parse(nil); err != nil {
		klog.Warning(err)
	}

	// Prevent log errors about logging before parsing.
	groups := templates.CommandGroups{
		{
			Message: "Cluster Master Init Commands:",
			Commands: []*cobra.Command{
				initmaster.CmdInitMaster(parentCommand),
				initmaster.CmdMasterDeInit(parentCommand),
			},
		},
		{
			Message: "Cluster Member Join/UnJoin Commands:",
			Commands: []*cobra.Command{
				membercurd.ClusterJoin(parentCommand),
				membercurd.ClusterUnJoin(parentCommand),
				membercurd.ClusterShow(parentCommand),
			},
		},
		{
			Message: "Cluster Floater Commands:",
			Commands: []*cobra.Command{
				floaterclient.CmdDoctor(parentCommand),
			},
		},
	}
	groups.Add(rootCmd)

	filters := []string{"options"}

	rootCmd.AddCommand(version.NewCmdVersion(parentCommand))
	templates.ActsAsRootCommand(rootCmd, filters, groups...)

	return rootCmd
}

func runHelp(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
