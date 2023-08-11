package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/clusterlink/cmd/network-manager/app/options"
	"github.com/kosmos.io/clusterlink/pkg/network-manager"
	"github.com/kosmos.io/clusterlink/pkg/scheme"
	"github.com/kosmos.io/clusterlink/pkg/sharedcli/klogflag"
)

func NewAgentCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "clusterlink-network-manager",
		Long: `Calculate the network configurations required for each node.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			if err := run(ctx, opts); err != nil {
				return err
			}
			return nil
		},
	}

	fss := cliflag.NamedFlagSets{}

	genericFlagSet := fss.FlagSet("generic")
	opts.AddFlags(genericFlagSet)

	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)

	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	return cmd
}

func run(ctx context.Context, opts *options.Options) error {
	config, err := clientcmd.BuildConfigFromFlags(opts.KubernetesOptions.Master, opts.KubernetesOptions.KubeConfig)
	if err != nil {
		panic(err)
	}
	config.QPS, config.Burst = opts.KubernetesOptions.QPS, opts.KubernetesOptions.Burst

	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Logger:                  klog.Background(),
		Scheme:                  scheme.NewSchema(),
		LeaderElection:          opts.LeaderElection.LeaderElect,
		LeaderElectionID:        opts.LeaderElection.ResourceName,
		LeaderElectionNamespace: opts.LeaderElection.ResourceNamespace,
	})
	if err != nil {
		return fmt.Errorf("failed to build controller manager: %v", err)
	}

	nodeConfigController := network_manager.Controller{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(network_manager.ControllerName),
	}
	if err = nodeConfigController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", network_manager.ControllerName, err)
	}

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start controller manager: %v", err)
	}

	return nil
}
