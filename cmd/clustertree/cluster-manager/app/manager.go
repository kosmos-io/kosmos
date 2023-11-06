package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/options"
	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/register"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/sharedcli/klogflag"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

func NewAgentCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "clustertree-cluster-manager",
		Long: `Cluster Resource Management and Synchronization.`,
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
	globalleafManager := leafUtils.GetGlobalLeafResourceManager()

	config, err := clientcmd.BuildConfigFromFlags(opts.KubernetesOptions.Master, opts.KubernetesOptions.KubeConfig)
	if err != nil {
		panic(err)
	}
	config.QPS, config.Burst = opts.KubernetesOptions.QPS, opts.KubernetesOptions.Burst

	configOptFunc := func(config *rest.Config) {
		config.QPS = opts.KubernetesOptions.QPS
		config.Burst = opts.KubernetesOptions.Burst
	}

	// init root client
	rootClient, err := utils.NewClientFromConfigPath(opts.KubernetesOptions.KubeConfig, configOptFunc)
	if err != nil {
		return fmt.Errorf("could not build clientset for root cluster: %v", err)
	}

	// init Kosmos client
	rootKosmosClient, err := utils.NewKosmosClientFromConfigPath(opts.KubernetesOptions.KubeConfig, configOptFunc)
	if err != nil {
		return fmt.Errorf("could not build kosmos clientset for root cluster: %v", err)
	}

	rootResourceManager := utils.NewResourceManager(rootClient, rootKosmosClient)
	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Logger:                  klog.Background(),
		Scheme:                  scheme.NewSchema(),
		LeaderElection:          opts.LeaderElection.LeaderElect,
		LeaderElectionID:        opts.LeaderElection.ResourceName,
		LeaderElectionNamespace: opts.LeaderElection.ResourceNamespace,
		MetricsBindAddress:      "0",
		HealthProbeBindAddress:  "0",
	})
	if err != nil {
		return fmt.Errorf("failed to build controller manager: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Errorf("Unable to create dynamicClient: %v", err)
		return err
	}

	rootControllerOptions := &register.RootControllerOptions{
		Ctx:                 ctx,
		Mgr:                 mgr,
		RootKosmosClient:    rootKosmosClient,
		RootDynamic:         dynamicClient,
		RootClient:          rootClient,
		Options:             opts,
		RootResourceManager: rootResourceManager,
		GlobalLeafManager:   globalleafManager,
	}
	err = register.AttachRootControllerToManager(rootControllerOptions)
	if err != nil {
		klog.Errorf("Unable to setup controller: %v", err)
		return err
	}

	go func() {
		if err = mgr.Start(ctx); err != nil {
			klog.Errorf("failed to start controller manager: %v", err)
		}
	}()

	rootResourceManager.InformerFactory.Start(ctx.Done())
	rootResourceManager.KosmosInformerFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), rootResourceManager.EndpointSliceInformer.HasSynced, rootResourceManager.ServiceInformer.HasSynced) {
		klog.Fatal("cluster manager: wait for informer factory failed")
	}

	<-ctx.Done()
	klog.Info("cluster node manager stopped.")
	return nil
}
