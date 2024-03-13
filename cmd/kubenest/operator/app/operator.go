package app

import (
	"context"
	"fmt"
	"github.com/kosmos.io/kosmos/cmd/kubenest/operator/app/options"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/kosmos"

	"github.com/spf13/cobra"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/sharedcli/klogflag"
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

	newscheme := scheme.NewSchema()
	err = apiextensionsv1.AddToScheme(newscheme)
	if err != nil {
		panic(err)
	}

	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Logger: klog.Background(),
		Scheme: newscheme,
		/*		LeaderElection:          opts.LeaderElection.LeaderElect,
				LeaderElectionID:        opts.LeaderElection.ResourceName,
				LeaderElectionNamespace: opts.LeaderElection.ResourceNamespace,*/
	})
	if err != nil {
		return fmt.Errorf("failed to build controller manager: %v", err)
	}

	VirtualClusterInitController := controller.VirtualClusterInitController{
		Client:        mgr.GetClient(),
		Config:        mgr.GetConfig(),
		EventRecorder: mgr.GetEventRecorderFor(constants.InitControllerName),
	}
	if err = VirtualClusterInitController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", constants.InitControllerName, err)
	}

	if opts.KosmosJoinController {
		KosmosJoinController := kosmos.KosmosJoinController{
			Client:                     mgr.GetClient(),
			EventRecorder:              mgr.GetEventRecorderFor(constants.KosmosJoinControllerName),
			KubeconfigPath:             opts.KubernetesOptions.KubeConfig,
			AllowNodeOwnbyMulticluster: opts.AllowNodeOwnbyMulticluster,
		}
		if err = KosmosJoinController.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error starting %s: %v", constants.KosmosJoinControllerName, err)
		}
	}

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start controller manager: %v", err)
	}

	return nil
}
