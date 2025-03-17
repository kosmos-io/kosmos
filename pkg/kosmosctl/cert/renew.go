package cert

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller"
	"github.com/kosmos.io/kosmos/pkg/scheme"
)

var RenewCertExample = templates.Examples(i18n.T(`
     # Renew cert, e.g:
     kosmosctl renew cert --kubeconfig=xxxx  --namespace=xxxx --name=xxxx
`))

type RenewOptions struct {
	Namespace      string
	Name           string
	KubeconfigPath string
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

	return cmd
}

func (o *RenewOptions) Complete() (err error) {
	return nil
}

func (o *RenewOptions) Validate() error {
	return nil
}

func (o *RenewOptions) Run() error {
	klog.V(4).Info("kosmos-io.tar.gz has been saved successfully. ")

	config, err := clientcmd.BuildConfigFromFlags("", o.KubeconfigPath)
	if err != nil {
		klog.Infof("Failed to build config: %v\n", err)
		return err
	}

	cli, err := client.New(config, client.Options{Scheme: scheme.NewSchema()})
	if err != nil {
		klog.Infof("Failed to create client: %v\n", err)
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Infof("Failed to create dynamic client: %v\n", err)
		return err
	}

	// 设置 CRD 的 Group、Version、Resource
	gvr := schema.GroupVersionResource{
		Group:    "kosmos.io",       // CRD 的 API 组
		Version:  "v1alpha1",        // CRD 的版本
		Resource: "virtualclusters", // 资源的复数名称
	}

	// 获取 CRD 资源
	unstructuredObj, err := dynamicClient.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get CRD resources: %v\n", err)
		return err
	}

	var virtualCluster v1alpha1.VirtualCluster
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, &virtualCluster)
	if err != nil {
		klog.Infof("Error converting to structured object: %v\n", err)
		return err
	}

	exec, err := controller.UpdateCertPhase(&virtualCluster, cli, config, &v1alpha1.KubeNestConfiguration{})
	if err != nil {
		panic(err)
	}

	err = exec.Execute()
	if err != nil {
		panic(err)
	}

	return nil
}
