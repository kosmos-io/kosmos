package initmaster

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/initmaster/ctlmaster"
	"github.com/kosmos.io/clusterlink/pkg/clusterlinkctl/util"
	"github.com/kosmos.io/clusterlink/pkg/version"
)

var (
	initLong = templates.LongDesc(`
		Install the Clusterlink control plane in a Kubernetes cluster.

		By default, the images and CRD tarball are downloaded remotely.
		For offline installation, you can set '--private-image-registry' and '--crds'.`)

	initExamples = templates.Examples(`
		# Install Clusterlink Operator and Clusterlink Controller Manager by default
		%[1]s init
		# Install Clusterlink Operator and Clusterlink Controller Manager by assigned  namespace
		%[1]s init --namespace NAMESPACE_NAME
		# Install Clusterlink Operator and Clusterlink Controller Manager by required  image
		%[1]s init --clusterlink-controller-image ghcr.io/kosmos-io/clusterlink/clusterlink-controller-manager:0.1.0 --clusterlink-operator-image  ghcr.io/kosmos-io/clusterlink/clusterlink-operator:0.1.0
		# Install Clusterlink Operator and Clusterlink Controller Manager by required  registry
		%[1]s init --private-image-registry ghcr.io/kosmos-io/clusterlink`)

	deinitExamples = templates.Examples(`
		# Deinit Clusterlink Operator and Clusterlink Controller Manager in all namespaces
		%[1]s dinit
		# Deinit Clusterlink Operator and Clusterlink Controller Manager in specifyed namespaces
		%[1]s init -n NAMESPACE_NAME`)
)

// CmdInitMaster Install Clusterlink on Kubernetes
func CmdInitMaster(parentCommand string) *cobra.Command {
	opts := ctlmaster.CommandInitOption{}
	cmd := &cobra.Command{
		Use:                   "init",
		Short:                 "Install the Clusterlink control plane in a Kubernetes cluster",
		Long:                  initLong,
		Example:               initExample(parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.InitKubeClient(); err != nil {
				return err
			}
			if err := opts.RunInit(parentCommand); err != nil {
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
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterRegistration,
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.ImageRegistry, "private-image-registry", "", "",
		"Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.  In addition, you still can use --kube-image-registry to specify the registry for Kubernetes's images.")
	//TODO: flags.StringSliceVar(&opts.PullSecrets, "image-pull-secrets", nil, "Image pull secrets are used to pull images from the private registry, could be secret list separated by comma (e.g '--image-pull-secrets PullSecret1,PullSecret2', the secrets should be pre-settled in the namespace declared by '--namespace')")
	// Kubernetes
	flags.StringVarP(&opts.Namespace, "namespace", "n", "clusterlink-system", "Kubernetes namespace")

	flags.StringVar(&opts.KubeConfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	//TODO: flags.StringVar(&opts.Context, "context", "", "The name of the kubeconfig context to use")

	flags.StringVarP(&opts.ClusterlinkOperatorImage, "clusterlink-operator-image", "", "", "Clusterlink Operator image")
	//TODO: flags.Int32VarP(&opts.ClusterlinkOperatorReplicas, "clusterlink-operator-replicas", "", 1, "Clusterlink Operator replica set")

	flags.StringVarP(&opts.ClusterlinkControllerImage, "clusterlink-controller-image", "", "", "Clusterlink Controller image")

	flags.Int32VarP(&opts.ClusterlinkControllerReplicas, "clusterlink-controller-replicas", "", 3, "Clusterlink Controller replica set")

	flags.StringVarP(&opts.CRDs, "crds", "", "", "clusterlink crd path")
	return cmd
}

func CmdMasterDeInit(parentCommand string) *cobra.Command {
	opts := ctlmaster.CommandInitOption{}
	cmd := &cobra.Command{
		Use:                   "deinit",
		Short:                 "Remove the Clusterlink control plane in a Kubernetes cluster",
		Long:                  initLong,
		Example:               deinitExample(parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.InitKubeClient(); err != nil {
				return err
			}
			if err := opts.RunDeInit(parentCommand); err != nil {
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
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterRegistration,
		},
	}
	flags := cmd.Flags()
	// Kubernetes
	flags.StringVarP(&opts.Namespace, "namespace", "n", "", "Kubernetes namespace")

	flags.StringVar(&opts.KubeConfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	//TODO: flags.StringVar(&opts.Context, "context", "", "The name of the kubeconfig context to use")

	return cmd
}

func initExample(parentCommand string) string {
	return fmt.Sprintf(initExamples, parentCommand, version.GetReleaseVersion().FirstMinorRelease())
}

func deinitExample(parentCommand string) string {
	return fmt.Sprintf(deinitExamples, parentCommand, version.GetReleaseVersion().FirstMinorRelease())
}
