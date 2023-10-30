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
	clusterManager "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager"
	controllers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/mcs"
	podcontrollers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pv"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pvc"
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
	globalleafManager := leafUtils.NewLeafResourceManager()

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

	// add cluster controller
	ClusterController := clusterManager.ClusterController{
		Root:                mgr.GetClient(),
		RootDynamic:         dynamicClient,
		RootClient:          rootClient,
		EventRecorder:       mgr.GetEventRecorderFor(clusterManager.ControllerName),
		Options:             opts,
		RootResourceManager: rootResourceManager,
		GlobalLeafManager:   globalleafManager,
	}
	if err = ClusterController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", clusterManager.ControllerName, err)
	}

	if opts.MultiClusterService {
		// add serviceExport controller
		ServiceExportController := mcs.ServiceExportController{
			RootClient:    mgr.GetClient(),
			EventRecorder: mgr.GetEventRecorderFor(mcs.ServiceExportControllerName),
			Logger:        mgr.GetLogger(),
		}
		if err = ServiceExportController.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error starting %s: %v", mcs.ServiceExportControllerName, err)
		}
	}

	if opts.DaemonSetController {
		daemonSetController := &GlobalDaemonSetService{
			opts:           opts,
			ctx:            ctx,
			defaultWorkNum: 1,
		}
		if err = daemonSetController.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error starting global daemonset : %v", err)
		}
	}

	// init rootPodController
	RootPodReconciler := podcontrollers.RootPodReconciler{
		GlobalLeafManager: globalleafManager,
		RootClient:        mgr.GetClient(),
		DynamicRootClient: dynamicClient,
		Options:           opts,
	}
	if err := RootPodReconciler.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting RootPodReconciler %s: %v", podcontrollers.RootPodControllerName, err)
	}

	rootPVCController := pvc.RootPVCController{
		RootClient:        mgr.GetClient(),
		GlobalLeafManager: globalleafManager,
	}
	if err := rootPVCController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting root pvc controller %v", err)
	}

	rootPVController := pv.RootPVController{
		RootClient:        mgr.GetClient(),
		GlobalLeafManager: globalleafManager,
	}
	if err := rootPVController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting root pv controller %v", err)
	}

	// init commonCOntroller
	for i, gvr := range controllers.SYNC_GVRS {
		commonController := controllers.SyncResourcesReconciler{
			GlobalLeafManager:    globalleafManager,
			GroupVersionResource: gvr,
			Object:               controllers.SYNC_OBJS[i],
			DynamicRootClient:    dynamicClient,
			// DynamicLeafClient:    clientDynamic,
			ControllerName: "async-controller-" + gvr.Resource,
			// Namespace:            cluster.Spec.Namespace,
		}
		if err := commonController.SetupWithManager(mgr, gvr); err != nil {
			klog.Errorf("Unable to create cluster node controller: %v", err)
			return err
		}
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
