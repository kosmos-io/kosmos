package app

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/options"
	clusterManager "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/mcs"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod"
	podcontrollers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pv"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pvc"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/svc"
	nodeserver "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/node-server"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/syncer"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/sharedcli/klogflag"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

func NewClusterManagerCommand(ctx context.Context) (*cobra.Command, error) {
	opts, err := options.NewOptions()
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:  "clustertree-cluster-manager",
		Long: `Cluster Resource Management and Synchronization.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			return leaderElectionRun(ctx, opts)
		},
	}

	fss := cliflag.NamedFlagSets{}

	genericFlagSet := fss.FlagSet("generic")
	opts.AddFlags(genericFlagSet)

	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)

	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	return cmd, nil
}

func leaderElectionRun(ctx context.Context, opts *options.Options) error {
	if !opts.LeaderElection.LeaderElect {
		return run(ctx, opts)
	}

	kubeConfig, err := clientcmd.BuildConfigFromFlags(opts.MasterURL, opts.KubeConfig)
	if err != nil {
		return err
	}
	utils.SetQPSBurst(kubeConfig, opts.KubernetesOptions)

	id, err := os.Hostname()
	if err != nil {
		return err
	}
	id += "_" + string(uuid.NewUUID())

	rl, err := resourcelock.NewFromKubeconfig(
		opts.LeaderElection.ResourceLock,
		opts.LeaderElection.ResourceNamespace,
		opts.LeaderElection.ResourceName,
		resourcelock.ResourceLockConfig{
			Identity: id,
		},
		kubeConfig,
		opts.LeaderElection.RenewDeadline.Duration,
	)
	if err != nil {
		return err
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		Name:          opts.LeaderElection.ResourceName,
		LeaseDuration: opts.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: opts.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   opts.LeaderElection.RetryPeriod.Duration,

		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.Warning("leader-election got, clustertree is awaking")
				_ = run(ctx, opts)
				os.Exit(0)
			},
			OnStoppedLeading: func() {
				klog.Warning("leader-election lost, clustertree is dying")
				os.Exit(0)
			},
		},
	})
	return nil
}

func run(ctx context.Context, opts *options.Options) error {
	globalLeafResourceManager := leafUtils.GetGlobalLeafResourceManager()
	globalLeafClientManager := leafUtils.GetGlobalLeafClientResourceManager()

	config, err := clientcmd.BuildConfigFromFlags(opts.KubernetesOptions.MasterURL, opts.KubernetesOptions.KubeConfig)
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
		Logger:                 klog.Background(),
		Scheme:                 scheme.NewSchema(),
		LeaderElection:         false,
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
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
	clusterController := clusterManager.ClusterController{
		Root:                      mgr.GetClient(),
		RootDynamic:               dynamicClient,
		RootClientset:             rootClient,
		EventRecorder:             mgr.GetEventRecorderFor(clusterManager.ControllerName),
		Options:                   opts,
		RootResourceManager:       rootResourceManager,
		GlobalLeafResourceManager: globalLeafResourceManager,
		GlobalLeafClientManager:   globalLeafClientManager,
	}
	if err = clusterController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", clusterManager.ControllerName, err)
	}

	if opts.MultiClusterService {
		// add serviceExport controller
		ServiceExportController := mcs.ServiceExportController{
			RootClient:         mgr.GetClient(),
			EventRecorder:      mgr.GetEventRecorderFor(mcs.ServiceExportControllerName),
			Logger:             mgr.GetLogger(),
			ReservedNamespaces: opts.ReservedNamespaces,
			RateLimiterOptions: opts.RateLimiterOpts,
			BackoffOptions:     opts.BackoffOpts,
		}
		if err = ServiceExportController.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error starting %s: %v", mcs.ServiceExportControllerName, err)
		}

		// add auto create mcs resources controller
		autoCreateMCSController := mcs.AutoCreateMCSController{
			RootClient:              mgr.GetClient(),
			EventRecorder:           mgr.GetEventRecorderFor(mcs.AutoCreateMCSControllerName),
			Logger:                  mgr.GetLogger(),
			AutoCreateMCSPrefix:     opts.AutoCreateMCSPrefix,
			RootKosmosClient:        rootKosmosClient,
			GlobalLeafManager:       globalLeafResourceManager,
			GlobalLeafClientManager: globalLeafClientManager,
			ReservedNamespaces:      opts.ReservedNamespaces,
		}
		if err = autoCreateMCSController.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error starting %s: %v", mcs.AutoCreateMCSControllerName, err)
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
	rootPodReconciler := podcontrollers.RootPodReconciler{
		GlobalLeafManager:       globalLeafResourceManager,
		GlobalLeafClientManager: globalLeafClientManager,

		RootClient:        mgr.GetClient(),
		DynamicRootClient: dynamicClient,
		Options:           opts,
	}
	if err := rootPodReconciler.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting rootPodReconciler %s: %v", podcontrollers.RootPodControllerName, err)
	}

	rootPVCController := pvc.RootPVCController{
		RootClient:              mgr.GetClient(),
		GlobalLeafManager:       globalLeafResourceManager,
		GlobalLeafClientManager: globalLeafClientManager,
	}
	if err := rootPVCController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting root pvc controller %v", err)
	}

	rootPVController := pv.RootPVController{
		RootClient:              mgr.GetClient(),
		GlobalLeafManager:       globalLeafResourceManager,
		GlobalLeafClientManager: globalLeafClientManager,
	}
	if err := rootPVController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting root pv controller %v", err)
	}

	RootPodSyncReconciler := pod.RootPodSyncReconciler{
		RootClient:              mgr.GetClient(),
		GlobalLeafManager:       globalLeafResourceManager,
		GlobalLeafClientManager: globalLeafClientManager,
	}
	if err := RootPodSyncReconciler.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting root podsync controller %v", err)
	}
	syncController := syncer.SyncController{
		RootManager:             mgr,
		GlobalLeafManager:       globalLeafResourceManager,
		GlobalLeafClientManager: globalLeafClientManager,
	}
	if err := syncController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting generic syncer controller %v", err)
	}

	if len(os.Getenv("USE-ONEWAY-STORAGE")) > 0 {
		onewayPVController := pv.OnewayPVController{
			Root:                    mgr.GetClient(),
			RootDynamic:             dynamicClient,
			GlobalLeafManager:       globalLeafResourceManager,
			GlobalLeafClientManager: globalLeafClientManager,
		}
		if err := onewayPVController.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error starting oneway pv controller %v", err)
		}

		onewayPVCController := pvc.OnewayPVCController{
			Root:                    mgr.GetClient(),
			RootDynamic:             dynamicClient,
			GlobalLeafManager:       globalLeafResourceManager,
			GlobalLeafClientManager: globalLeafClientManager,
		}
		if err := onewayPVCController.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error starting oneway pvc controller %v", err)
		}
	}

	// init commonController
	for i, gvr := range controllers.SYNC_GVRS {
		commonController := controllers.SyncResourcesReconciler{
			GlobalLeafManager:       globalLeafResourceManager,
			GlobalLeafClientManager: globalLeafClientManager,
			GroupVersionResource:    gvr,
			Object:                  controllers.SYNC_OBJS[i],
			DynamicRootClient:       dynamicClient,
			// DynamicLeafClient:    clientDynamic,
			ControllerName: "async-controller-" + gvr.Resource,
			// Namespace:            cluster.Spec.Namespace,
		}
		if err := commonController.SetupWithManager(mgr, gvr); err != nil {
			klog.Errorf("Unable to create cluster node controller: %v", err)
			return err
		}
	}

	// init direct sync service and endpointslice controller
	if opts.DirectClusterService {
		simpleSyncServiceController := &svc.SimpleSyncServiceController{
			RootClient:              mgr.GetClient(),
			GlobalLeafManager:       globalLeafResourceManager,
			GlobalLeafClientManager: globalLeafClientManager,
			AutoCreateMCSPrefix:     opts.AutoCreateMCSPrefix,
			ReservedNamespaces:      opts.ReservedNamespaces,
		}
		if err := simpleSyncServiceController.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error starting %s: %v", svc.SimpleSyncServiceControllerName, err)
		}

		simpleSyncEpsController := &svc.SimpleSyncEPSController{
			RootClient:              mgr.GetClient(),
			GlobalLeafManager:       globalLeafResourceManager,
			GlobalLeafClientManager: globalLeafClientManager,
			AutoCreateMCSPrefix:     opts.AutoCreateMCSPrefix,
			ReservedNamespaces:      opts.ReservedNamespaces,
			BackoffOptions:          opts.BackoffOpts,
		}
		if err := simpleSyncEpsController.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error starting %s: %v", svc.SimpleSyncEPSControllerName, err)
		}
	}
	go func() {
		if err = mgr.Start(ctx); err != nil {
			klog.Errorf("failed to start controller manager: %v", err)
		}
	}()

	nodeServer := nodeserver.NodeServer{
		RootClient:              mgr.GetClient(),
		GlobalLeafManager:       globalLeafResourceManager,
		GlobalLeafClientManager: globalLeafClientManager,
	}
	go func() {
		if err := nodeServer.Start(ctx, opts); err != nil {
			klog.Errorf("failed to start node server: %v", err)
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
