package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/json"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/cmd/clusterlink/floater/app/options"
	networkmanager "github.com/kosmos.io/kosmos/pkg/clusterlink/agent-manager/network-manager"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network"
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

	cols, _, err := term.TerminalSize(cmd.OutOrStdout())
	if err != nil {
		klog.Errorf("terminal size error: %s", err)
	}
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)

	return cmd
}

func Run(_ context.Context, _ *options.Options) error {
	enableAnalysis, err := strconv.ParseBool(os.Getenv("ENABLE_ANALYSIS"))
	if err != nil {
		klog.Errorf("env variable read error: %s", err)
	}

	if enableAnalysis {
		if err = collectNetworkConfig(); err != nil {
			return err
		}
	}

	port := os.Getenv("PORT")
	klog.Infof("PORT: ", port)
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

	err = server.ListenAndServe()
	if err != nil {
		klog.Errorf("launch server error: %s", err)
		panic(err)
	}

	return nil
}

func collectNetworkConfig() error {
	var nodeConfigSpecByte []byte

	net := network.NewNetWork(false)
	nManager := networkmanager.NewNetworkManager(net)
	klog.Infof("Starting collect network config, create network manager...")

	nodeConfigSpec, err := nManager.LoadSystemConfig()
	if err != nil {
		klog.Errorf("nodeConfigSpec query error: %s", err)
	}
	klog.Infof("load system config into nodeConfigSpec succeeded, nodeConfigSpec: [", nodeConfigSpec, "]")

	nodeConfigSpecByte, err = json.Marshal(nodeConfigSpec)
	if err != nil {
		klog.Errorf("nodeConfigSpec marshal error: %s", err)
	}
	klog.Infof("marshal nodeConfigSpec into bytes succeeded, nodeConfigSpecByte: [", nodeConfigSpecByte, "]")

	filePath := filepath.Join(os.Getenv("HOME"), "nodeconfig.json")
	err = os.WriteFile(filePath, nodeConfigSpecByte, os.ModePerm)
	if err != nil {
		klog.Errorf("nodeconfig.json write error: %s", err)
	}
	klog.Infof("nodeconfig.json has been written, filePath: [", filePath, "]")

	return nil
}
