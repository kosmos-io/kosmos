package membercurd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ClusterJoin Install Clusterlink on Kubernetes
func ClusterJoin(parentCommand string) *cobra.Command {
	opts := CommandMemberOption{}

	var cmd = &cobra.Command{
		Use:                   "join",
		Short:                 "Join a cluster as a Member",
		Long:                  `Join a cluster as a Member `,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.InitKubeClient(); err != nil {
				return err
			}
			if err := opts.Validate(args); err != nil {
				return err
			}
			if err := opts.AddKubeCluster(); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.KubeConfig, "kubeconfig", "", "absolute path to the kubeconfig file")

	flags.StringVarP(&opts.ImageRegistry, "private-image-registry", "", "",
		"Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.  In addition, you still can use --kube-image-registry to specify the registry for Kubernetes's images.")
	flags.StringVarP(&opts.CNIPlugin, "cni", "", "calico", "")
	flags.StringVarP(&opts.NetworkType, "networktype", "", "p2p", "")
	flags.StringVarP(&opts.MemberKubeConfig, "memberkubeconfig", "", "",
		"absolute path to the member cluster kubeconfig file")

	return cmd
}

func ClusterUnJoin(parentCommand string) *cobra.Command {
	opts := CommandMemberOption{}

	var cmd = &cobra.Command{
		Use:   "unjoin",
		Short: "Unjoin a Member cluster",
		Long:  `Unjoin a Member cluster.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.InitKubeClient(); err != nil {
				return err
			}
			if err := opts.Validate(args); err != nil {
				return err
			}
			if err := opts.DelKubeCluster(); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.KubeConfig, "kubeconfig", "", "absolute path to the kubeconfig file")

	return cmd
}

func ClusterShow(parentCommand string) *cobra.Command {
	opts := CommandMemberOption{}

	var cmd = &cobra.Command{
		Use: "show",

		Short: "show clusters",
		Long:  `show cluster.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.InitKubeClient(); err != nil {
				return err
			}
			if err := opts.ShowKubeCluster(); err != nil {
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

	flags := cmd.Flags()
	flags.StringVar(&opts.KubeConfig, "kubeconfig", "", "absolute path to the kubeconfig file")

	return cmd
}
