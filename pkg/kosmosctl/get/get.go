package get

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctlget "k8s.io/kubectl/pkg/cmd/get"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/utils/pointer"

	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	ClustersGroupResource     = "clusters.kosmos.io"
	ClusterNodesGroupResource = "clusternodes.kosmos.io"
	NodeConfigsGroupResource  = "nodeconfigs.kosmos.io"
)

type CommandGetOptions struct {
	Cluster     string
	ClusterNode string

	Namespace string

	GetOptions *ctlget.GetOptions
}

var newF ctlutil.Factory

// NewCmdGet Display resources from the Kosmos control plane.
func NewCmdGet(f ctlutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCommandGetOptions(streams)

	cmd := &cobra.Command{
		Use:                   fmt.Sprintf("get [(-o|--output=)%s] (TYPE[.VERSION][.GROUP] [NAME | -l label] | TYPE[.VERSION][.GROUP]/NAME ...) [flags]", strings.Join(o.GetOptions.PrintFlags.AllowedFormats(), "|")),
		Short:                 i18n.T("Display resources from the Kosmos control plane"),
		Long:                  "",
		Example:               "",
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(f, cmd, args))
			ctlutil.CheckErr(o.Validate())
			ctlutil.CheckErr(o.Run(f, cmd, args))
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&o.Namespace, "namespace", "n", utils.DefaultNamespace, "If present, the namespace scope for this CLI request.")
	flags.StringVar(&o.Cluster, "cluster", utils.DefaultClusterName, "Specify a cluster, the default is the control cluster.")
	o.GetOptions.PrintFlags.AddFlags(cmd)

	return cmd
}

// NewCommandGetOptions returns a CommandGetOptions.
func NewCommandGetOptions(streams genericclioptions.IOStreams) *CommandGetOptions {
	getOptions := ctlget.NewGetOptions("kosmosctl", streams)
	return &CommandGetOptions{
		GetOptions: getOptions,
	}
}

func (o *CommandGetOptions) Complete(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
	if o.Cluster != utils.DefaultClusterName {
		controlConfig, err := f.ToRESTConfig()
		if err != nil {
			return err
		}

		rootClient, err := versioned.NewForConfig(controlConfig)
		if err != nil {
			return err
		}
		cluster, err := rootClient.KosmosV1alpha1().Clusters().Get(context.TODO(), o.Cluster, metav1.GetOptions{})
		if err != nil {
			return err
		}

		leafConfig, err := clientcmd.RESTConfigFromKubeConfig(cluster.Spec.Kubeconfig)
		if err != nil {
			return fmt.Errorf("kosmosctl get complete error, load leaf cluster kubeconfig failed: %s", err)
		}

		leafClient, err := kubernetes.NewForConfig(leafConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl get complete error, generate leaf cluster client failed: %s", err)
		}

		kosmosControlSA, err := util.GenerateServiceAccount(manifest.KosmosControlServiceAccount, manifest.ServiceAccountReplace{
			Namespace: o.Namespace,
		})
		if err != nil {
			return fmt.Errorf("kosmosctl get complete error, generate kosmos serviceaccount failed: %s", err)
		}
		expirationSeconds := int64(600)
		leafToken, err := leafClient.CoreV1().ServiceAccounts(kosmosControlSA.Namespace).CreateToken(
			context.TODO(), kosmosControlSA.Name, &authenticationv1.TokenRequest{
				Spec: authenticationv1.TokenRequestSpec{
					ExpirationSeconds: &expirationSeconds,
				},
			}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("kosmosctl get complete error, list leaf cluster secret failed: %s", err)
		}

		configFlags := genericclioptions.NewConfigFlags(false)
		configFlags.APIServer = &leafConfig.Host
		configFlags.BearerToken = &leafToken.Status.Token
		configFlags.Insecure = pointer.Bool(true)
		configFlags.Namespace = &o.Namespace

		newF = ctlutil.NewFactory(configFlags)

		err = o.GetOptions.Complete(newF, cmd, args)
		if err != nil {
			return fmt.Errorf("kosmosctl get complete error, options failed: %s", err)
		}

		o.GetOptions.Namespace = o.Namespace
	} else {
		err := o.GetOptions.Complete(f, cmd, args)
		if err != nil {
			return fmt.Errorf("kosmosctl get complete error, options failed: %s", err)
		}

		o.GetOptions.Namespace = o.Namespace
	}

	return nil
}

func (o *CommandGetOptions) Validate() error {
	err := o.GetOptions.Validate()
	if err != nil {
		return fmt.Errorf("kosmosctl get validate error, options failed: %s", err)
	}

	return nil
}

func (o *CommandGetOptions) Run(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "cluster", "clusters":
		args[0] = ClustersGroupResource
	case "clusternode", "clusternodes":
		args[0] = ClusterNodesGroupResource
	case "nodeconfig", "nodeconfigs":
		args[0] = NodeConfigsGroupResource
	}

	if o.Cluster != utils.DefaultClusterName {
		err := o.GetOptions.Run(newF, cmd, args)
		if err != nil {
			return fmt.Errorf("kosmosctl get run error, options failed: %s", err)
		}
	} else {
		err := o.GetOptions.Run(f, cmd, args)
		if err != nil {
			return fmt.Errorf("kosmosctl get run error, options failed: %s", err)
		}
	}

	return nil
}
