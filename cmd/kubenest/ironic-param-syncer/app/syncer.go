package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/kosmos/cmd/kubenest/ironic-param-syncer/app/config"
	"github.com/kosmos.io/kosmos/cmd/kubenest/ironic-param-syncer/app/options"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	controller "github.com/kosmos.io/kosmos/pkg/kubenest/controller/ironic.parameter.controller"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/sharedcli/klogflag"
)

func NewIronicParameterSyncerCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "virtual-cluster-operator",
		Long: `create virtual kubernetes control plane with VirtualCluster`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runCommand(ctx, opts); err != nil {
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

	c.RestConfig = kubeConfig
	c.Client = client
	c.LeaderElection = opts.LeaderElection

	return c, nil
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
	})
	if err != nil {
		return fmt.Errorf("failed to build controller manager: %v", err)
	}

	ironicParameterController := controller.IronicParameterController{
		Client:        mgr.GetClient(),
		Config:        mgr.GetConfig(),
		EventRecorder: mgr.GetEventRecorderFor(constants.InitControllerName),
	}
	if err = ironicParameterController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", constants.InitControllerName, err)
	}

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start controller manager: %v", err)
	}

	return nil
}
