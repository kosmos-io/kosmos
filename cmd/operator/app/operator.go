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

	"github.com/kosmos.io/clusterlink/cmd/operator/app/options"
	"github.com/kosmos.io/clusterlink/pkg/operator"
	"github.com/kosmos.io/clusterlink/pkg/scheme"
	"github.com/kosmos.io/clusterlink/pkg/sharedcli"
	"github.com/kosmos.io/clusterlink/pkg/sharedcli/klogflag"
	"github.com/kosmos.io/clusterlink/pkg/utils"
)

// NewOperatorCommand creates a *cobra.Command object with default parameters
func NewOperatorCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "clusterlink-operator",
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
	clientset, err := kubernetes.NewForConfig(restConfig)
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
		cm, err := clientset.CoreV1().ConfigMaps(utils.NamespaceClusterLinksystem).Get(context.Background(), opts.ExternalKubeConfigName, metav1.GetOptions{})
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

	clusterNodeController := operator.Reconciler{
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
