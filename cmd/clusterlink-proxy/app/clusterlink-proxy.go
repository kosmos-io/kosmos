package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/cmd/clusterlink-proxy/app/options"
)

// NewClusterLinkProxyCommand creates a *cobra.Command object with default parameters
func NewClusterLinkProxyCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "proxy",
		Long: `The proxy starts a apiserver for agent access the backend proxy`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate options
			/*
				if errs := opts.Validate(); len(errs) != 0 {
					return errs.ToAggregate()
				}
			*/
			if err := run(ctx, opts); err != nil {
				return err
			}
			return nil
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}
	namedFlagSets := opts.Flags()

	fs := cmd.Flags()
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	cols, _, err := term.TerminalSize(cmd.OutOrStdout())
	if err != nil {
		klog.Warning("term.TerminalSize err: %v", err)
	} else {
		cliflag.SetUsageAndHelpFunc(cmd, namedFlagSets, cols)
	}

	return cmd
}

func run(ctx context.Context, opts *options.Options) error {
	config, err := opts.Config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	return server.GenericAPIServer.PrepareRun().Run(ctx.Done())
}
