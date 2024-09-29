package app

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kosmos.io/kosmos/cmd/kubenest/operator/app/config"
	"github.com/kosmos.io/kosmos/cmd/kubenest/operator/app/options"
	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller"
	endpointscontroller "github.com/kosmos.io/kosmos/pkg/kubenest/controller/endpoints.sync.controller"
	glnodecontroller "github.com/kosmos.io/kosmos/pkg/kubenest/controller/global.node.controller"
	kosmos "github.com/kosmos.io/kosmos/pkg/kubenest/controller/kosmos"
	vcnodecontroller "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/sharedcli/klogflag"
)

func NewVirtualClusterOperatorCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "virtual-cluster-operator",
		Long: `create virtual kubernetes control plane with VirtualCluster`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(ctx, opts)
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

func runCommand(ctx context.Context, opts *options.Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	kc, err := SetupConfig(opts)
	if err != nil {
		return err
	}
	return run(ctx, kc)
}

func SetupConfig(opts *options.Options) (*config.Config, error) {
	c := &config.Config{}

	var koc v1alpha1.KubeNestConfiguration
	if len(opts.ConfigFile) != 0 {
		ko, err := loadConfig(opts.ConfigFile)
		if err != nil {
			return nil, err
		}
		koc = *ko
	} else {
		ko := &v1alpha1.KubeNestConfiguration{}
		ko.KubeNestType = v1alpha1.KubeInKube
		ko.KosmosKubeConfig.AllowNodeOwnbyMulticluster = false
		ko.KubeInKubeConfig.ForceDestroy = opts.DeprecatedOptions.KubeInKubeConfig.ForceDestroy
		ko.KubeInKubeConfig.ETCDUnitSize = opts.DeprecatedOptions.KubeInKubeConfig.ETCDUnitSize
		ko.KubeInKubeConfig.ETCDStorageClass = opts.DeprecatedOptions.KubeInKubeConfig.ETCDStorageClass
		ko.KubeInKubeConfig.AdmissionPlugins = opts.DeprecatedOptions.KubeInKubeConfig.AdmissionPlugins
		ko.KubeInKubeConfig.AnpMode = opts.DeprecatedOptions.KubeInKubeConfig.AnpMode
		ko.KubeInKubeConfig.APIServerReplicas = opts.DeprecatedOptions.KubeInKubeConfig.APIServerReplicas
		ko.KubeInKubeConfig.ClusterCIDR = opts.DeprecatedOptions.KubeInKubeConfig.ClusterCIDR

		koc = *ko
	}

	fillInForDefault(c, koc)
	printKubeNestConfiguration(koc)

	kubeconfigStream, err := os.ReadFile(opts.KubernetesOptions.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("read kubeconfig file failed: %v", err)
	}

	// Prepare kube config.
	kubeConfig, err := createKubeConfig(opts)
	if err != nil {
		return nil, err
	}

	// Prepare kube clients.
	client, err := createClients(kubeConfig)
	if err != nil {
		return nil, err
	}

	c.KubeconfigStream = kubeconfigStream
	c.RestConfig = kubeConfig
	c.Client = client
	c.LeaderElection = opts.LeaderElection
	c.KubeNestOptions = koc

	return c, nil
}

// TODO
func printKubeNestConfiguration(_ v1alpha1.KubeNestConfiguration) {

}

// TODO
func fillInForDefault(_ *config.Config, _ v1alpha1.KubeNestConfiguration) {

}

func loadConfig(file string) (*v1alpha1.KubeNestConfiguration, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	// The UniversalDecoder runs defaulting and returns the internal type by default.
	obj, gvk, err := scheme.Codecs.UniversalDecoder().Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}
	if cfgObj, ok := obj.(*v1alpha1.KubeNestConfiguration); ok {
		return cfgObj, nil
	}
	return nil, fmt.Errorf("couldn't decode as KubeNestConfiguration, got %s: ", gvk)
}

// createClients creates a kube client and an event client from the given kubeConfig
func createClients(kubeConfig *restclient.Config) (clientset.Interface, error) {
	client, err := clientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// createKubeConfig creates a kubeConfig from the given config and masterOverride.
func createKubeConfig(opts *options.Options) (*restclient.Config, error) {
	if len(opts.KubernetesOptions.KubeConfig) == 0 && len(opts.KubernetesOptions.Master) == 0 {
		klog.Warning("Neither --kubeconfig nor --master was specified. Using default API client. This might not work.")
	}

	// This creates a client, first loading any specified kubeconfig
	// file, and then overriding the Master flag, if non-empty.
	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: opts.KubernetesOptions.KubeConfig},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: opts.KubernetesOptions.Master}}).ClientConfig()
	if err != nil {
		return nil, err
	}

	kubeConfig.DisableCompression = true
	kubeConfig.QPS = opts.KubernetesOptions.QPS
	kubeConfig.Burst = opts.KubernetesOptions.Burst

	return kubeConfig, nil
}

func startEndPointsControllers(mgr manager.Manager) error {
	coreEndPointsController := endpointscontroller.CoreDNSController{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(constants.GlobalNodeControllerName),
	}

	if err := coreEndPointsController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", endpointscontroller.CoreDNSSyncControllerName, err)
	}

	KonnectivityEndPointsController := endpointscontroller.KonnectivityController{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(constants.GlobalNodeControllerName),
	}

	if err := KonnectivityEndPointsController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", endpointscontroller.KonnectivitySyncControllerName, err)
	}

	APIServerExternalSyncController := endpointscontroller.APIServerExternalSyncController{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(constants.GlobalNodeControllerName),
	}

	if err := APIServerExternalSyncController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", endpointscontroller.APIServerExternalSyncControllerName, err)
	}

	return nil
}

func run(ctx context.Context, config *config.Config) error {
	newscheme := scheme.NewSchema()
	err := apiextensionsv1.AddToScheme(newscheme)
	if err != nil {
		panic(err)
	}

	mgr, err := controllerruntime.NewManager(config.RestConfig, controllerruntime.Options{
		Logger:                  klog.Background(),
		Scheme:                  newscheme,
		LeaderElection:          config.LeaderElection.LeaderElect,
		LeaderElectionID:        config.LeaderElection.ResourceName,
		LeaderElectionNamespace: config.LeaderElection.ResourceNamespace,
		LivenessEndpointName:    "/healthz",
		ReadinessEndpointName:   "/readyz",
		HealthProbeBindAddress:  ":8081",
	})
	if err != nil {
		return fmt.Errorf("failed to build controller manager: %v", err)
	}

	err = mgr.AddHealthzCheck("healthz", healthz.Ping)
	if err != nil {
		return fmt.Errorf("failed to build healthz: %v", err)
	}

	err = mgr.AddReadyzCheck("readyz", healthz.Ping)
	if err != nil {
		return fmt.Errorf("failed to build readyz: %v", err)
	}

	hostKubeClient, err := kubernetes.NewForConfig(config.RestConfig)
	if err != nil {
		return fmt.Errorf("could not create clientset: %v", err)
	}

	kosmosClient, err := versioned.NewForConfig(config.RestConfig)
	if err != nil {
		return fmt.Errorf("could not create clientset: %v", err)
	}

	VirtualClusterInitController := controller.VirtualClusterInitController{
		Client:          mgr.GetClient(),
		Config:          mgr.GetConfig(),
		EventRecorder:   mgr.GetEventRecorderFor(constants.InitControllerName),
		RootClientSet:   hostKubeClient,
		KosmosClient:    kosmosClient,
		KubeNestOptions: &config.KubeNestOptions,
	}
	if err = VirtualClusterInitController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", constants.InitControllerName, err)
	}

	GlobalNodeController := glnodecontroller.GlobalNodeController{
		Client:        mgr.GetClient(),
		RootClientSet: hostKubeClient,
		KosmosClient:  kosmosClient,
		EventRecorder: mgr.GetEventRecorderFor(constants.GlobalNodeControllerName),
	}

	if err = GlobalNodeController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", constants.GlobalNodeControllerName, err)
	}

	if err := startEndPointsControllers(mgr); err != nil {
		return err
	}

	VirtualClusterNodeController := vcnodecontroller.NewNodeController(
		mgr.GetClient(),
		hostKubeClient,
		mgr.GetEventRecorderFor(constants.NodeControllerName),
		kosmosClient,
		&config.KubeNestOptions,
	)

	if err = VirtualClusterNodeController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", constants.NodeControllerName, err)
	}

	if config.KubeNestOptions.KubeNestType == v1alpha1.KosmosKube {
		KosmosJoinController := kosmos.KosmosJoinController{
			Client:                     mgr.GetClient(),
			EventRecorder:              mgr.GetEventRecorderFor(constants.KosmosJoinControllerName),
			KubeConfig:                 config.RestConfig,
			KubeconfigStream:           config.KubeconfigStream,
			AllowNodeOwnbyMulticluster: config.KubeNestOptions.KosmosKubeConfig.AllowNodeOwnbyMulticluster,
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
