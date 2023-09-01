package app

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	klogv3 "k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/cmd/knode-manager/app/config"
	"github.com/kosmos.io/kosmos/cmd/knode-manager/app/options"
	"github.com/kosmos.io/kosmos/pkg/clusterrouter/knodemanager"
)

func NewKosmosNodeManagerCommand(ctx context.Context) *cobra.Command {
	opts, _ := options.NewKosmosNodeOptions()
	cmd := &cobra.Command{
		Use: "kosmosnode-manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := opts.Config()
			if err != nil {
				return err
			}

			if err := Run(ctx, config); err != nil {
				return err
			}
			return nil
		},
	}

	namedFlagSets := opts.Flags()

	fs := cmd.Flags()
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cliflag.SetUsageAndHelpFunc(cmd, namedFlagSets, cols)

	return cmd
}

func Run(ctx context.Context, c *config.Config) error {
	knManager := knodemanager.NewManager(c)
	if !c.LeaderElection.LeaderElect {
		knManager.Run(c.WorkerNumber, ctx.Done())
		return nil
	}

	id, err := os.Hostname()
	if err != nil {
		return err
	}
	id += "_" + string(uuid.NewUUID())

	rl, err := resourcelock.NewFromKubeconfig(
		c.LeaderElection.ResourceLock,
		c.LeaderElection.ResourceNamespace,
		c.LeaderElection.ResourceName,
		resourcelock.ResourceLockConfig{
			Identity: id,
		},
		c.KubeConfig,
		c.LeaderElection.RenewDeadline.Duration,
	)
	if err != nil {
		return fmt.Errorf("failed to create resource lock: %w", err)
	}

	var done chan struct{}
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Name: c.LeaderElection.ResourceName,

		Lock:          rl,
		LeaseDuration: c.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: c.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   c.LeaderElection.RetryPeriod.Duration,

		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				done = make(chan struct{})
				defer close(done)

				stopCh := ctx.Done()
				knManager.Run(c.WorkerNumber, stopCh)
			},
			OnStoppedLeading: func() {
				klogv3.Info("leaderelection lost")
				if done != nil {
					<-done
				}
			},
		},
	})
	return nil
}
