package app

import (
	"context"
	"flag"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/clusterlink/cmd/controller-manager/app/options"
	ctrlcontext "github.com/kosmos.io/clusterlink/pkg/controllers/context"
	"github.com/kosmos.io/clusterlink/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/clusterlink/pkg/scheme"
	"github.com/kosmos.io/clusterlink/pkg/sharedcli/klogflag"
)

var (
	Controllers = make(ctrlcontext.Initializers)

	// ControllersDisabledByDefault is the set of Controllers which is disabled by default
	ControllersDisabledByDefault = sets.New[string]()
)

func init() {
	Controllers["cluster"] = startClusterController
	Controllers["node"] = startNodeController
	Controllers["calicoIPPool"] = startCalicoPoolController
	Controllers["nodecidr"] = startNodeCIDRController
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
	opts.AddFlags(genericFlagSet, Controllers.ControllerNames(), sets.List(ControllersDisabledByDefault))

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

	controllerManager, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: scheme.NewSchema(),
	})
	if err != nil {
		klog.Errorf("failed to build controller manager: %v", err)
		return err
	}

	//TODO 整理这块
	controlPanelConfig, err := clientcmd.BuildConfigFromFlags("", opts.ControlPanelConfig)
	if err != nil {
		klog.Fatalf("build controlpanel config err: %v", err)
		panic(err)
	}

	clusterLinkClient, err := versioned.NewForConfig(controlPanelConfig)
	if err != nil {
		klog.Fatalf("Unable to create clusterlinkClient: %v", err)
		panic(err)
	}

	controller := NewController(clusterLinkClient, controllerManager, opts)
	err = controller.Start(ctx)
	if err != nil {
		return err
	}

	return nil
}

func setupControllers(mgr ctrl.Manager, opts *options.ControllerManagerOptions, ctx context.Context) []ctrlcontext.CleanFunc {
	controlPanelConfig, err := clientcmd.BuildConfigFromFlags("", opts.ControlPanelConfig)
	if err != nil {
		klog.Fatalf("build controlpanel config err: %v", err)
		panic(err)
	}

	clusterLinkClient, err := versioned.NewForConfig(controlPanelConfig)
	if err != nil {
		klog.Fatalf("Unable to create clusterlinkClient: %v", err)
		panic(err)
	}

	controllerContext := ctrlcontext.Context{
		Mgr: mgr,
		Opts: ctrlcontext.Options{
			Controllers:        opts.Controllers,
			ControlPanelConfig: controlPanelConfig,
			ClusterName:        opts.ClusterName,
			RateLimiterOpts:    opts.RateLimiterOpts,
		},
		Ctx:               ctx,
		ClusterLinkClient: clusterLinkClient,
	}

	cleanFuns, err := Controllers.StartControllers(controllerContext, ControllersDisabledByDefault)
	if err != nil {
		klog.Fatalf("error starting Controllers: %v", err)
		panic(err)
	}

	return cleanFuns
}
