package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/kosmos/cmd/koslet/app/config"
	"github.com/kosmos.io/kosmos/cmd/koslet/app/options"
	kubeletManager "github.com/kosmos.io/kosmos/pkg/koslet/manager/controller"
	"github.com/kosmos.io/kosmos/pkg/scheme"
)

func NewKosletCommand(ctx context.Context) *cobra.Command {
	opts, _ := options.NewKosletOptions()

	cmd := &cobra.Command{
		Use:  "kosmos-koslet",
		Long: `The kosmos koslet runs on each node and provides node management capabilities.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := opts.Config()
			if err != nil {
				return err
			}

			if err := run(ctx, config); err != nil {
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

func run(ctx context.Context, c *config.Config) error {
	config, err := clientcmd.BuildConfigFromFlags("", c.KubeConfigPath)
	if err != nil {
		panic(err)
	}
	config.QPS, config.Burst = c.KubeAPIQPS, c.KubeAPIBurst

	if err != nil {
		return fmt.Errorf("could not build clientset for koslet manager: %v", err)
	}

	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Logger:                  klog.Background(),
		Scheme:                  scheme.NewSchema(),
		LeaderElection:          c.LeaderElection.LeaderElect,
		LeaderElectionID:        c.LeaderElection.ResourceName,
		LeaderElectionNamespace: c.LeaderElection.ResourceNamespace,
		MetricsBindAddress:      "0",
		HealthProbeBindAddress:  "0",
	})
	if err != nil {
		return fmt.Errorf("failed to build controller manager: %v", err)
	}

	// add koslet Pod controller
	KosletPodController := kubeletManager.KubeletController{
		Master:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(kubeletManager.KubeletControllerName),
	}
	if err = KosletPodController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", kubeletManager.KubeletControllerName, err)
	}

	if err = mgr.Start(ctx); err != nil {
		klog.Errorf("failed to start koslet manager: %v", err)
	}
	<-ctx.Done()
	klog.Info("koslet manager stopped.")
	return nil
}
