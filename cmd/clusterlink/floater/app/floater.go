package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/cmd/clusterlink/floater/app/options"
	"github.com/kosmos.io/kosmos/pkg/sharedcli"
	"github.com/kosmos.io/kosmos/pkg/sharedcli/klogflag"
)

func NewFloaterCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "clusterlink-floater",
		Long: `Environment for executing commands`,
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

	cols, _, err := term.TerminalSize(cmd.OutOrStdout())
	if err != nil {
		klog.Errorf("terminal size error: %s", err)
	}
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)

	return cmd
}

func run(_ context.Context, _ *options.Options) error {
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8889"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("OK"))
		if err != nil {
			klog.Errorf("response writer error: %s", err)
		}
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		ReadHeaderTimeout: 3 * time.Second,
	}

	err := server.ListenAndServe()
	if err != nil {
		klog.Errorf("lanch server error: %s", err)
		panic(err)
	}

	return nil
}
