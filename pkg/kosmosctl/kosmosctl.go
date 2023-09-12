package kosmosctl

import (
	"flag"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/floater"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/get"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/install"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/join"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/uninstall"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/unjoin"
)

// DefaultConfigFlags It composes the set of values necessary for obtaining a REST client config with default values set.
var DefaultConfigFlags = genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)

// NewKosmosCtlCommand creates the `kosmosctl` command with arguments.
func NewKosmosCtlCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:   "kosmosctl",
		Short: i18n.T("kosmosctl controls a Kubernetes cluster network"),
		Long:  templates.LongDesc(`kosmosctl controls a Kubernetes cluster network.`),
		RunE:  runHelp,
	}

	klog.InitFlags(flag.CommandLine)

	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	cmds.PersistentFlags().AddFlagSet(pflag.CommandLine)

	if err := flag.CommandLine.Parse(nil); err != nil {
		klog.Warning(err)
	}

	f := ctlutil.NewFactory(DefaultConfigFlags)
	ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	groups := templates.CommandGroups{
		{
			Message: "Basic Commands:",
			Commands: []*cobra.Command{
				get.NewCmdGet(f, ioStreams),
			},
		},
		{
			Message: "Install/UnInstall Commands:",
			Commands: []*cobra.Command{
				install.NewCmdInstall(f),
				uninstall.NewCmdUninstall(f, ioStreams),
			},
		},
		{
			Message: "Cluster Member Join/UnJoin Commands:",
			Commands: []*cobra.Command{
				join.NewCmdJoin(f),
				unjoin.NewCmdUnJoin(f),
			},
		},
		{
			Message: "Cluster Doctor/Floater Commands:",
			Commands: []*cobra.Command{
				floater.NewCmdDoctor(),
			},
		},
	}
	groups.Add(cmds)

	return cmds
}

func runHelp(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
