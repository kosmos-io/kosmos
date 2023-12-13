package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bep/debounce"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/kosmos/cmd/clusterlink/agent/app/options"
	linkagent "github.com/kosmos.io/kosmos/pkg/clusterlink/agent-manager"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network"
	kosmosclientset "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	kosmosinformer "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/sharedcli"
	"github.com/kosmos.io/kosmos/pkg/sharedcli/klogflag"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

// NewAgentCommand creates a *cobra.Command object with default parameters
func NewAgentCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "clusterlink-agent",
		Long: `Configure the network based on clusternodes and clusters`,
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

	cols, h, err := term.TerminalSize(cmd.OutOrStdout())
	if err != nil {
		klog.Warning(err, h)
	}
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)

	return cmd
}

func run(ctx context.Context, opts *options.Options) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %s", err.Error())
	}

	if err = network.CreateGlobalNetIptablesChains(); err != nil {
		return fmt.Errorf("failed to create clusterlink iptables chains: %s", err.Error())
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme.NewSchema(),
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		klog.Fatalf("failed to build controller manager123: %v", err)
		return err
	}

	clusterlinkClientset, err := kosmosclientset.NewForConfig(restConfig)
	if err != nil {
		klog.Fatalf("Unable to create clusterlink clientset: %v", err)
		return err
	}

	factory := kosmosinformer.NewSharedInformerFactory(clusterlinkClientset, 0)
	nodeConfigLister := factory.Kosmos().V1alpha1().NodeConfigs().Lister()
	clusterLister := factory.Kosmos().V1alpha1().Clusters().Lister()

	clusterNodeController := linkagent.Reconciler{
		Scheme:           mgr.GetScheme(),
		NodeConfigLister: nodeConfigLister,
		ClusterLister:    clusterLister,
		NodeName:         os.Getenv(utils.EnvNodeName),
		ClusterName:      os.Getenv(utils.EnvClusterName),
		NetworkManager:   linkagent.NetworkManager(),
		DebounceFunc:     debounce.New(5 * time.Second),
	}
	if err = clusterNodeController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Unable to create cluster node controller: %v", err)
		return err
	}

	autoDetectController := linkagent.AutoDetectReconciler{
		Client:      mgr.GetClient(),
		NodeName:    os.Getenv(utils.EnvNodeName),
		ClusterName: os.Getenv(utils.EnvClusterName),
	}

	if err = autoDetectController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Unable to create auto detect controller: %v", err)
		return err
	}

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	// go wait.UntilWithContext(ctx, func(ctx context.Context) {
	// 	if err := clusterNodeController.Cleanup(); err != nil {
	// 		klog.Warningf("An error was encountered while cleaning: %v", err)
	// 	}
	// }, opts.CleanPeriod)
	go clusterNodeController.StartTimer(ctx)

	if err := mgr.Start(ctx); err != nil {
		klog.Errorf("controller manager exits unexpectedly: %v", err)
		return err
	}

	return nil
}
