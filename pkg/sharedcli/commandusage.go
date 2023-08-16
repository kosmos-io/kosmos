package sharedcli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

const (
	usageFmt = "Usage:\n  %s\n"
)

// generatesAvailableSubCommands generates command's subcommand information which
// is usually part of a help message. E.g.:
//
// Available Commands:
//
//	cfn-controller-manager completion                      generate the autocompletion script for the specified shell
//	cfn-controller-manager help                            Help about any command
//	cfn-controller-manager version                         Print the version information.
func generatesAvailableSubCommands(cmd *cobra.Command) []string {
	if !cmd.HasAvailableSubCommands() {
		return nil
	}

	info := []string{"\nAvailable Commands:"}
	for _, sub := range cmd.Commands() {
		if !sub.Hidden {
			info = append(info, fmt.Sprintf("  %s %-30s  %s", cmd.CommandPath(), sub.Name(), sub.Short))
		}
	}
	return info
}

// SetUsageAndHelpFunc set both usage and help function.
func SetUsageAndHelpFunc(cmd *cobra.Command, fss cliflag.NamedFlagSets, cols int) {
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		_, err := fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		if err != nil {
			klog.Warning("fmt.Fprintf err: %v", err)
		}
		if cmd.HasAvailableSubCommands() {
			_, err := fmt.Fprintf(cmd.OutOrStderr(), "%s\n", strings.Join(generatesAvailableSubCommands(cmd), "\n"))
			if err != nil {
				klog.Warning("fmt.Fprintf err: %v", err)
			}
		}
		cliflag.PrintSections(cmd.OutOrStderr(), fss, cols)
		return nil
	})

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		if err != nil {
			klog.Warning("fmt.Fprintf err: %v", err)
		}
		if cmd.HasAvailableSubCommands() {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n", strings.Join(generatesAvailableSubCommands(cmd), "\n"))
			if err != nil {
				klog.Warning("fmt.Fprintf err: %v", err)
			}
		}
		cliflag.PrintSections(cmd.OutOrStdout(), fss, cols)
	})
}
