package logs

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctllogs "k8s.io/kubectl/pkg/cmd/logs"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/utils/pointer"

	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

var (
	logsLong = templates.LongDesc(i18n.T(`
		Print the logs for a container in a pod or specified resource from the specified cluster. 
		If the pod has only one container, the container name is optional.`))

	logsExample = templates.Examples(i18n.T(`
		# Return logs from pod, e.g:
		kosmosctl logs pod-name --cluster cluster-name
		
		# Return logs from pod of special container, e.g:
		kosmosctl logs pod-name --cluster cluster-name -c container-name`))
)

type CommandLogsOptions struct {
	Cluster string

	Namespace string

	LogsOptions *ctllogs.LogsOptions
}

func NewCmdLogs(f ctlutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCommandLogsOptions(streams)

	cmd := &cobra.Command{
		Use:                   "logs [-f] [-p] (POD | TYPE/NAME) [-c CONTAINER] (--cluster CLUSTER_NAME)",
		Short:                 i18n.T("Display resources from the Kosmos control plane"),
		Long:                  logsLong,
		Example:               logsExample,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(f, cmd, args))
			ctlutil.CheckErr(o.Validate())
			ctlutil.CheckErr(o.Run())
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&o.Namespace, "namespace", "n", utils.DefaultNamespace, "If present, the namespace scope for this CLI request.")
	flags.StringVar(&o.Cluster, "cluster", utils.DefaultClusterName, "Specify a cluster, the default is the control cluster.")
	o.LogsOptions.AddFlags(cmd)

	return cmd
}

func NewCommandLogsOptions(streams genericclioptions.IOStreams) *CommandLogsOptions {
	logsOptions := ctllogs.NewLogsOptions(streams, false)
	return &CommandLogsOptions{
		LogsOptions: logsOptions,
	}
}

func (o *CommandLogsOptions) Complete(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("kosmosctl logs complete error, load leaf cluster kubeconfig failed: %s", err)
	}

	leafClient, err := kubernetes.NewForConfig(leafConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl logs complete error, generate leaf cluster client failed: %s", err)
	}

	kosmosControlSA, err := util.GenerateServiceAccount(manifest.KosmosControlServiceAccount, manifest.ServiceAccountReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl logs complete error, generate kosmos serviceaccount failed: %s", err)
	}
	expirationSeconds := int64(600)
	leafToken, err := leafClient.CoreV1().ServiceAccounts(kosmosControlSA.Namespace).CreateToken(
		context.TODO(), kosmosControlSA.Name, &authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				ExpirationSeconds: &expirationSeconds,
			},
		}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("kosmosctl logs complete error, list leaf cluster secret failed: %s", err)
	}

	configFlags := genericclioptions.NewConfigFlags(false)
	configFlags.APIServer = &leafConfig.Host
	configFlags.BearerToken = &leafToken.Status.Token
	configFlags.Insecure = pointer.Bool(true)
	configFlags.Namespace = &o.Namespace

	o.LogsOptions.Namespace = o.Namespace

	newF := ctlutil.NewFactory(configFlags)
	err = o.LogsOptions.Complete(newF, cmd, args)
	if err != nil {
		return fmt.Errorf("kosmosctl logs complete error, options failed: %s", err)
	}

	return nil
}

func (o *CommandLogsOptions) Validate() error {
	err := o.LogsOptions.Validate()
	if err != nil {
		return fmt.Errorf("kosmosctl logs validate error, options failed: %s", err)
	}

	return nil
}

func (o *CommandLogsOptions) Run() error {
	err := o.LogsOptions.RunLogs()
	if err != nil {
		return fmt.Errorf("kosmosctl logs run error, options failed: %s", err)
	}

	return nil
}
