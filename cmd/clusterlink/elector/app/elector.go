package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/cmd/clusterlink/elector/app/options"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/elector"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/sharedcli"
	"github.com/kosmos.io/kosmos/pkg/sharedcli/klogflag"
)

// NewElectorCommand creates a *cobra.Command object with default parameters
func NewElectorCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use: "elector",
		// TODO add some thing here
		Long: ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
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

	fss := cliflag.NamedFlagSets{}

	genericFlagSet := fss.FlagSet("generic")
	opts.AddFlags(genericFlagSet)

	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)

	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, err := term.TerminalSize(cmd.OutOrStdout())
	if err != nil {
		klog.Warning("term.TerminalSize err: %v", err)
	} else {
		sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	}

	return cmd
}

func run(ctx context.Context, opts *options.Options) error {
	memberClusterConfig, err := clientcmd.BuildConfigFromFlags("", opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %+v", err)
	}

	controlpanelConfig, err := clientcmd.BuildConfigFromFlags("", opts.ControlPanelConfig)
	if err != nil {
		return fmt.Errorf("error building controlpanelConfig: %+v", err)
	}
	controlpanelClient, err := versioned.NewForConfig(controlpanelConfig)
	if err != nil {
		return fmt.Errorf("error  building controlpanelClient: %+v", err)
	}

	elector := elector.NewElector(controlpanelClient)
	leaderElectionClient, err := kubernetes.NewForConfig(rest.AddUserAgent(memberClusterConfig, "leader-election"))
	if err != nil {
		return err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("unable to get hostname: %v", err)
	}
	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id := hostname + "_" + string(uuid.NewUUID())
	rl, err := resourcelock.New(opts.LeaderElection.ResourceLock,
		opts.LeaderElection.ResourceNamespace,
		opts.LeaderElection.ResourceName,
		leaderElectionClient.CoreV1(),
		leaderElectionClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: id,
		})
	if err != nil {
		return fmt.Errorf("couldn't create resource lock: %v", err)
	}
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: opts.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: opts.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   opts.LeaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.Infof("become leader start set gateway role")
				for {
					select {
					case <-ctx.Done():
						klog.Infof("elector stop")
					default:
						err := elector.EnsureGateWayRole()
						if err != nil {
							klog.Errorf("set gateway role failure: %v, retry after 10 sec.", err)
							time.Sleep(3 * time.Second)
						} else {
							timeToRecheck := 3 * time.Second
							klog.V(4).Infof("ensure gateway role success, recheck after %d sec.", int(timeToRecheck))
							time.Sleep(timeToRecheck)
						}
					}
				}
			},
			OnStoppedLeading: func() {
				klog.Fatalf("leaderelection lost")
			},
		},
	})
	return nil
}
