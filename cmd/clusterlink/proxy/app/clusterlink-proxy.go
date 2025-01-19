package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	genericapiserver "k8s.io/apiserver/pkg/server"
	cliflag "k8s.io/component-base/cli/flag"

	"github.com/kosmos.io/kosmos/cmd/clusterlink/proxy/app/options"
	"github.com/kosmos.io/kosmos/pkg/sharedcli/klogflag"
	profileflag "github.com/kosmos.io/kosmos/pkg/sharedcli/profileflag"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

// NewClusterLinkProxyCommand creates a *cobra.Command object with default parameters
func NewClusterLinkProxyCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  utils.KosmosClusrerLinkRroxyComponentName,
		Long: `starts a server for agent kube-apiserver`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := opts.Validate(); err != nil {
				return err
			}
			return run(ctx, opts)
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

	flags := cmd.Flags()

	fss := cliflag.NamedFlagSets{}
	genericFlagSet := fss.FlagSet("generic")
	opts.AddFlags(genericFlagSet)

	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)

	flags.AddFlagSet(genericFlagSet)
	flags.AddFlagSet(logsFlagSet)

	return cmd
}

func run(ctx context.Context, opts *options.Options) error {
	// pprof
	profileflag.ListenAndServe(opts.ProfileOpts)

	config, err := opts.Config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	server.GenericAPIServer.AddPostStartHookOrDie("start-proxy-controller", func(context genericapiserver.PostStartHookContext) error {
		go func() {
			config.ExtraConfig.ProxyController.Run(context.StopCh, 1)
		}()
		return nil
	})

	server.GenericAPIServer.AddPostStartHookOrDie("start-apiserver-informer", func(context genericapiserver.PostStartHookContext) error {
		config.ExtraConfig.KosmosInformerFactory.Start(context.StopCh)
		return nil
	})

	return server.GenericAPIServer.PrepareRun().Run(ctx.Done())
}
