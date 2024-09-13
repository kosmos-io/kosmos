package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/kosmos/cmd/clusterlink/clusterlink-operator/app/options"
	clusterlinkoperator "github.com/kosmos.io/kosmos/pkg/clusterlink/clusterlink-operator"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/sharedcli"
	"github.com/kosmos.io/kosmos/pkg/sharedcli/klogflag"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

// NewLinkOperatorCommand creates a *cobra.Command object with default parameters
func NewLinkOperatorCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "kosmos-clusterlink-operator",
		Long: `Deploy Kosmos clusterlink components according to the cluster`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			return Run(ctx, opts)
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

func Run(ctx context.Context, opts *options.Options) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %s", err.Error())
	}
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("error get kubeclient: %v", err)
	}

	var controlPanelKubeConfig *clientcmdapi.Config
	if len(opts.ControlPanelKubeConfig) > 0 {
		controlPanelKubeConfig, err = clientcmd.LoadFromFile(opts.ControlPanelKubeConfig)
		if err != nil {
			return fmt.Errorf("failed to load controlpanelKubeConfig: %v", err)
		}
	} else {
		// try get kubeconfig from configmap
		cm, err := clientSet.CoreV1().ConfigMaps(utils.DefaultNamespace).Get(context.Background(), opts.ExternalKubeConfigName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to configmap %s: %v", opts.ExternalKubeConfigName, err)
		}
		kubeConfigYAML, ok := cm.Data["kubeconfig"]
		if !ok {
			return errors.New("config key not found in ConfigMap")
		}
		config, err := clientcmd.Load([]byte(kubeConfigYAML))
		if err != nil {
			return err
		}
		controlPanelKubeConfig = config
	}

	c, err := clientcmd.BuildConfigFromFlags("", opts.ControlPanelKubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %s", err.Error())
	}
	mgr, err := ctrl.NewManager(c, ctrl.Options{
		Scheme: scheme.NewSchema(),
	})

	if err != nil {
		klog.Errorf("failed to build controller manager: %v", err)
		return err
	}

	clusterNodeController := clusterlinkoperator.Reconciler{
		Scheme:                 mgr.GetScheme(),
		ControlPanelKubeConfig: controlPanelKubeConfig,
		ClusterName:            os.Getenv(utils.EnvClusterName),
		Options:                opts,
	}

	if err = clusterNodeController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Unable to create cluster node controller: %v", err)
		return err
	}
	klog.Infoln("Starting operator controller manager...")
	if err := mgr.Start(ctx); err != nil {
		klog.Errorf("controller manager exits unexpectedly: %v", err)
		return err
	}
	return nil
}
