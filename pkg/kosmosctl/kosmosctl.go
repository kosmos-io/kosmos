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
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/image"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/install"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/join"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/logs"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/rsmigrate"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/uninstall"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/unjoin"
)

// DefaultConfigFlags It composes the set of values necessary for obtaining a REST client config with default values set.
var DefaultConfigFlags = genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)

// NewKosmosCtlCommand creates the `kosmosctl` command with arguments.
func NewKosmosCtlCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:   "kosmosctl",
		Short: i18n.T("kosmosctl controls the Kosmos cluster manager"),
		Long:  templates.LongDesc(`kosmosctl controls the Kosmos cluster manager.`),
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
				install.NewCmdInstall(),
				uninstall.NewCmdUninstall(),
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
			Message: "Troubleshooting and Debugging Commands:",
			Commands: []*cobra.Command{
				logs.NewCmdLogs(f, ioStreams),
				floater.NewCmdCheck(),
				floater.NewCmdAnalysis(f),
			},
		}, {
			Message: "Cluster Resource Import/Export Commands:",
			Commands: []*cobra.Command{
				rsmigrate.NewCmdImport(f),
				rsmigrate.NewCmdExport(),
			},
		},
		{
			Message: "Image Pull/Push commands",
			Commands: []*cobra.Command{
				image.NewCmdImage(),
			},
		},
	}
	groups.Add(cmds)

	return cmds
}

func runHelp(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
