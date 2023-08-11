package app

import (
	"context"
	"flag"
	"sync"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"cnp.io/clusterlink/cmd/controller-manager/app/options"
	ctrlcontext "cnp.io/clusterlink/pkg/controllers/context"
	"cnp.io/clusterlink/pkg/controllers/nodecidr"
	"cnp.io/clusterlink/pkg/generated/clientset/versioned"
	"cnp.io/clusterlink/pkg/scheme"
	"cnp.io/clusterlink/pkg/sharedcli/klogflag"
)

var (
	controllers = make(ctrlcontext.Initializers)

	// controllersDisabledByDefault is the set of controllers which is disabled by default
	controllersDisabledByDefault = sets.New[string]()

	stopOnce sync.Once
)

func init() {
	controllers["cluster"] = startClusterController
	controllers["node"] = startNodeController
	controllers["calicoIPPool"] = startCalicoPoolController
}

// NewControllerManagerCommand creates a *cobra.Command object with default parameters
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {
	opts := options.NewControllerManagerOptions()

	cmd := &cobra.Command{
		Use:  "clusterlink-controller-manager",
		Long: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			return Run(ctx, opts)
		},
	}

	fss := cliflag.NamedFlagSets{}

	genericFlagSet := fss.FlagSet("generic")
	// Add the flag(--kubeconfig) that is added by controller-runtime
	// (https://github.com/kubernetes-sigs/controller-runtime/blob/v0.11.1/pkg/client/config/config.go#L39),
	// and update the flag usage.
	genericFlagSet.AddGoFlagSet(flag.CommandLine)
	genericFlagSet.Lookup("kubeconfig").Usage = "Path to clusterlink control plane kubeconfig file."
	opts.AddFlags(genericFlagSet, controllers.ControllerNames(), sets.List(controllersDisabledByDefault))

	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)

	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, err := term.TerminalSize(cmd.OutOrStdout())
	if err != nil {
		klog.Warning("fmt.Fprintf err: %v", err)
	}
	cliflag.SetUsageAndHelpFunc(cmd, fss, cols)

	return cmd
}

// Run runs the ControllerManagerOptions.
func Run(ctx context.Context, opts *options.ControllerManagerOptions) error {
	config, err := ctrl.GetConfig()
	if err != nil {
		panic(err)
	}

	// ToDo 创建Manager时配置好自定义资源Schema、选举配置、存活检查和指标监控
	controllerManager, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: scheme.NewSchema(),
	})
	if err != nil {
		klog.Errorf("failed to build controller manager: %v", err)
		return err
	}

	setupControllers(controllerManager, opts, ctx.Done())

	if err := controllerManager.Start(ctx); err != nil {
		klog.Errorf("controller manager exits unexpectedly: %v", err)
		return err
	}

	return nil
}

func setupControllers(mgr ctrl.Manager, opts *options.ControllerManagerOptions, stopChan <-chan struct{}) {
	controlPanelConfig, err := clientcmd.BuildConfigFromFlags("", opts.ControlPanelConfig)
	if err != nil {
		klog.Fatalf("build controlpanel config err: %v", err)
		panic(err)
	}

	controllerContext := ctrlcontext.Context{
		Mgr: mgr,
		Opts: ctrlcontext.Options{
			Controllers:        opts.Controllers,
			ControlPanelConfig: controlPanelConfig,
			ClusterName:        opts.ClusterName,
		},
		StopChan: stopChan,
	}

	if err := controllers.StartControllers(controllerContext, controllersDisabledByDefault); err != nil {
		klog.Fatalf("error starting controllers: %v", err)
		panic(err)
	}

	config, err := clientcmd.BuildConfigFromFlags("", opts.ControlPanelConfig)
	if err != nil {
		klog.Fatalf("Unable to create controlPanelConfig: %v", err)
		panic(err)
	}
	clusterLinkClient, err := versioned.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Unable to create clusterLinkClient: %v", err)
		panic(err)
	}
	nodeCIDRCtl := nodecidr.NewNodeCIDRController(mgr.GetConfig(), opts.ClusterName, clusterLinkClient, opts.RateLimiterOpts, stopChan)
	if err := mgr.Add(nodeCIDRCtl); err != nil {
		klog.Fatalf("Failed to setup node CIDR Controller: %v", err)
		panic(err)
	}

	// Ensure the InformerManager stops when the stop channel closes
	go func() {
		<-stopChan
		StopInstance()
	}()
}

func StopInstance() {
	stopOnce.Do(func() {
		close(make(chan struct{}))
	})
}
