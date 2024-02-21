package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/kosmos.io/kosmos/cmd/webhook/app/options"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/sharedcli/klogflag"
	kosmoswebhook "github.com/kosmos.io/kosmos/pkg/webhook"
)

func NewWebhookCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "kosmos-webhook",
		Long: `TODO`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			if err := Run(ctx, opts); err != nil {
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

	return cmd
}

func Run(ctx context.Context, opts *options.Options) error {
	config, err := clientcmd.BuildConfigFromFlags(opts.KubernetesOptions.Master, opts.KubernetesOptions.KubeConfig)
	if err != nil {
		panic(err)
	}
	config.QPS, config.Burst = opts.KubernetesOptions.QPS, opts.KubernetesOptions.Burst

	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Logger: klog.Background(),
		Scheme: scheme.NewSchema(),
		WebhookServer: &webhook.Server{
			Host:     opts.WebhookServerOptions.Host,
			Port:     opts.WebhookServerOptions.Port,
			CertDir:  opts.WebhookServerOptions.CertDir,
			CertName: opts.WebhookServerOptions.CertName,
			KeyName:  opts.WebhookServerOptions.KeyName,
		},
		MetricsBindAddress:      "0",
		HealthProbeBindAddress:  "0",
		LeaderElection:          opts.LeaderElection.LeaderElect,
		LeaderElectionID:        opts.LeaderElection.ResourceName,
		LeaderElectionNamespace: opts.LeaderElection.ResourceNamespace,
	})
	if err != nil {
		klog.Errorf("failed to build webhook server: %v", err)
		return err
	}

	hookServer := mgr.GetWebhookServer()
	// register an Admission Webhook
	hookServer.Register("/validate-delete-pod", &webhook.Admission{Handler: &kosmoswebhook.PodValidator{
		Client:  mgr.GetClient(),
		Options: opts.PodValidatorOptions,
	}})
	hookServer.WebhookMux.Handle("/health", http.StripPrefix("/health", &healthz.Handler{}))

	if err := mgr.Start(ctx); err != nil {
		klog.Errorf("failed to start webhook manager: %v", err)
		return err
	}

	return nil
}
