package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/kosmos/cmd/clusterlink/network-manager/app/options"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/sharedcli/klogflag"
	"github.com/kosmos.io/kosmos/pkg/treeoperator"
)

func NewVirtualClusterOperatorCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "virtual-cluster-operator",
		Long: `create virtual kubernetes control plane with VirtualCluster`,
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

	VirtualClusterInitController := treeoperator.VirtualClusterInitController{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(treeoperator.InitControllerName),
	}
	if err = VirtualClusterInitController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", treeoperator.InitControllerName, err)
	}

	VirtualClusterJoinController := treeoperator.VirtualClusterJoinController{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(treeoperator.JoinControllerName),
	}
	if err = VirtualClusterJoinController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", treeoperator.JoinControllerName, err)
	}

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start controller manager: %v", err)
	}

	return nil
}
