package cert

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

var RenewCertExample = templates.Examples(i18n.T(`
     # Renew cert, e.g:
     kosmosctl renew cert --kubeconfig=xxxx  --namespace=xxxx --name=xxxx --agent-user=xxxx --agent-pass=xxxx
`))

type RenewOptions struct {
	Namespace      string
	Name           string
	KubeconfigPath string
	NodeAgentOptions
}

type NodeAgentOptions struct {
	WebUser string
	WebPass string
}

func NewCmdRenewCert() *cobra.Command {
	o := &RenewOptions{}
	cmd := &cobra.Command{
		Use:                   "cert",
		Short:                 i18n.T("renew cert for virtual cluster. "),
		Long:                  "",
		Example:               RenewCertExample,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete())
			ctlutil.CheckErr(o.Validate())
			ctlutil.CheckErr(o.Run())
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&o.Namespace, "namespace", "e", "", "namespace of vc")
	flags.StringVarP(&o.Name, "name", "n", "", "name of vc")
	flags.StringVarP(&o.KubeconfigPath, "kubeconfig", "k", "", "kubeconfig path of host cluster")
	flags.StringVarP(&o.WebUser, "agent-user", "u", "", "user of node agent")
	flags.StringVarP(&o.WebPass, "agent-pass", "p", "", "password of node agent")
	return cmd
}

func (o *RenewOptions) Complete() (err error) {
	return nil
}

func (o *RenewOptions) Validate() error {
	if len(o.WebPass) == 0 {
		return fmt.Errorf("web pass is required")
	}

	if len(o.WebUser) == 0 {
		return fmt.Errorf("use pass is required")
	}
	if len(o.KubeconfigPath) == 0 {
		return fmt.Errorf("kubeconfig path is required")
	}
	if len(o.Namespace) == 0 {
		return fmt.Errorf("namespace is required")
	}
	if len(o.Name) == 0 {
		return fmt.Errorf("name is required")
	}
	return nil
}

func (o *RenewOptions) initEnv() {
	os.Setenv("KUBECONFIG", o.KubeconfigPath)
	os.Setenv("WEB_USER", o.WebUser)
	os.Setenv("WEB_PASS", o.WebPass)
}

func (o *RenewOptions) Run() error {
	r, err := NewCertOption(o)
	o.initEnv()
	if err != nil {
		return err
	}
	return Do(r)
}

func Do(r *Option) error {
	err := RunTask([]TaskFunc{
		RunCheckEnvironment,
		RunBackupSecrets,
		RunReCreateCertAndKubeConfig,
		UpdateKubeProxyConfig,
		RestartVirtualControlPlanePod,
		RestartVirtualWorkerKubelet,
		RestartVirtualPod,
	}, r)
	if err != nil {
		return err
	}
	klog.Infof("############ renew cert success!!!!")
	return nil
}
