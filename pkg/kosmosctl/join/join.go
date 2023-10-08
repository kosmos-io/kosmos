package join

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/version"
)

var joinExample = templates.Examples(i18n.T(`
        # Join cluster resource from a directory containing cluster.yaml, e.g: 
        kosmosctl join -f cluster.yaml --master-kubeconfig=[master-kubeconfig] --cluster-kubeconfig=[cluster-kubeconfig]
        
        # Join cluster resource without master-kubeconfig, e.g: 
        kosmosctl join -f cluster.yaml --cluster-kubeconfig=[cluster-kubeconfig]

        # Join knode resource from a directory containing knode.yaml, e.g: 
        kosmosctl join -f knode.yaml --master-kubeconfig=[master-kubeconfig] --knode-kubeconfig=[knode-kubeconfig]

        # Join knode resource without master-kubeconfig, e.g: 
        kosmosctl join -f knode.yaml --knode-kubeconfig=[knode-kubeconfig]
`))

type CommandJoinOptions struct {
	Module   string
	File     string
	Resource *unstructured.Unstructured

	MasterKubeConfig  string
	ClusterKubeConfig string
	KnodeKubeConfig   string

	Client        kubernetes.Interface
	DynamicClient *dynamic.DynamicClient
}

// NewCmdJoin join resource to Kosmos control plane.
func NewCmdJoin(f ctlutil.Factory) *cobra.Command {
	o := &CommandJoinOptions{}

	cmd := &cobra.Command{
		Use:                   "join",
		Short:                 i18n.T("Join resource to Kosmos control plane"),
		Long:                  "",
		Example:               joinExample,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(f))
			ctlutil.CheckErr(o.Validate())
			ctlutil.CheckErr(o.Run())
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&o.File, "file", "f", "", "Absolute path to the resource file.")
	err := cmd.MarkFlagRequired("file")
	if err != nil {
		fmt.Printf("kosmosctl join cmd error, MarkFlagRequired failed: %s", err)
	}
	flags.StringVar(&o.MasterKubeConfig, "master-kubeconfig", "", "Absolute path to the master kubeconfig file.")
	flags.StringVar(&o.ClusterKubeConfig, "cluster-kubeconfig", "", "Absolute path to the cluster kubeconfig file.")
	flags.StringVar(&o.KnodeKubeConfig, "knode-kubeconfig", "", "Absolute path to the knode kubeconfig file.")

	return cmd
}

func (o *CommandJoinOptions) Complete(f ctlutil.Factory) error {
	var masterConfig *restclient.Config
	var err error

	if len(o.MasterKubeConfig) > 0 {
		masterConfig, err = clientcmd.BuildConfigFromFlags("", o.MasterKubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate masterConfig failed: %s", err)
		}
	} else {
		masterConfig, err = f.ToRESTConfig()
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate masterConfig failed: %s", err)
		}
	}

	o.DynamicClient, err = dynamic.NewForConfig(masterConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate dynamic client failed: %s", err)
	}

	if len(o.ClusterKubeConfig) > 0 {
		clusterConfig, err := clientcmd.BuildConfigFromFlags("", o.ClusterKubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate clusterConfig failed: %s", err)
		}

		o.Client, err = kubernetes.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate basic client failed: %v", err)
		}
	}

	if len(o.KnodeKubeConfig) > 0 {
		knodeConfig, err := clientcmd.BuildConfigFromFlags("", o.KnodeKubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate knodeConfig failed: %s", err)
		}

		o.Client, err = kubernetes.NewForConfig(knodeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate basic client failed: %v", err)
		}
	}

	return nil
}

func (o *CommandJoinOptions) Validate() error {
	yamlFileByte, err := os.ReadFile(o.File)
	if err != nil {
		return fmt.Errorf("kosmosctl join validate warning, read yaml file failed: %s", err)
	}

	decoder := k8syaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err = decoder.Decode(yamlFileByte, nil, obj)
	if err != nil {
		return fmt.Errorf("kosmosctl join validate warning, decode failed: %s", err)
	}

	switch obj.GetKind() {
	case "Cluster":
		_, err = o.DynamicClient.Resource(util.ClusterGVR).Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("kosmosctl join validate warning, clsuter already exists: %s", err)
			}
		}
		o.Module = "clusterlink"
		o.Resource = obj
	case "Knode":
		_, err = o.DynamicClient.Resource(util.KnodeGVR).Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
		if err != nil && apierrors.IsAlreadyExists(err) {
			if apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("kosmosctl join validate warning, knode already exists: %s", err)
			}
		}
		o.Module = "clustertree"
		o.Resource = obj
	}

	return nil
}

func (o *CommandJoinOptions) Run() error {
	switch o.Module {
	case "clusterlink":
		err := o.runCluster()
		if err != nil {
			return err
		}
	case "clustertree":
		err := o.runKnode()
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *CommandJoinOptions) runCluster() error {
	klog.Info("Start registering cluster to kosmos control plane...")
	// 1. create cluster in master
	_, err := o.DynamicClient.Resource(util.ClusterGVR).Namespace("").Create(context.TODO(), o.Resource, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("(cluster) kosmosctl join run warning, create cluster failed: %s", err)
	}

	// 2. create namespace in member
	namespace := &corev1.Namespace{}
	namespace.Name = util.DefaultNamespace
	_, err = o.Client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster namespace) kosmosctl join run warning, create namespace failed: %s", err)
	}

	// 3. create secret in member
	masterKubeConfig, err := os.ReadFile(o.MasterKubeConfig)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster secret) kosmosctl join run warning, read masterconfig failed: %s", err)
	}
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.ControlPanelSecretName,
			Namespace: util.DefaultNamespace,
		},
		Data: map[string][]byte{
			"kubeconfig": masterKubeConfig,
		},
	}
	_, err = o.Client.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster secret) kosmosctl join run warning, create secret failed: %s", err)
	}

	// 4. create rbac in member
	clusterRole, err := util.GenerateClusterRole(manifest.ClusterlinkClusterRole, nil)
	if err != nil {
		return fmt.Errorf("(cluster rbac) kosmosctl join run warning, generate clusterrole failed: %s", err)
	}
	_, err = o.Client.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster rbac) kosmosctl join run warning, create clusterrole failed: %s", err)
	}
	clusterRoleBinding, err := util.GenerateClusterRoleBinding(manifest.ClusterlinkClusterRoleBinding, nil)
	if err != nil {
		return fmt.Errorf("(cluster rbac) kosmosctl join run warning, generate clusterrolebinding failed: %s", err)
	}
	_, err = o.Client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster rbac) kosmosctl join run warning, create clusterrolebinding failed: %s", err)
	}

	// 5. create operator in member
	serviceAccount, err := util.GenerateServiceAccount(manifest.ClusterlinkOperatorServiceAccount, nil)
	if err != nil {
		return fmt.Errorf("(cluster operator) kosmosctl join run warning, generate serviceaccount failed: %s", err)
	}
	_, err = o.Client.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster operator) kosmosctl join run warning, create serviceaccount failed: %s", err)
	}

	deployment, err := util.GenerateDeployment(manifest.ClusterlinkOperatorDeployment, manifest.ClusterlinkDeploymentReplace{
		Version:     version.GetReleaseVersion().PatchRelease(),
		ClusterName: o.Resource.GetName(),
	})
	if err != nil {
		return fmt.Errorf("(cluster operator) kosmosctl join run warning, generate deployment failed: %s", err)
	}
	_, err = o.Client.AppsV1().Deployments(deployment.Namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster operator) kosmosctl join run warning, create deployment failed: %s", err)
	}
	if err = util.WaitDeploymentReady(o.Client, deployment, 120); err != nil {
		return fmt.Errorf("(cluster operator) kosmosctl join run warning, create deployment failed: %s", err)
	} else {
		klog.Info("Cluster registration successful.")
	}

	return nil
}

func (o *CommandJoinOptions) runKnode() error {
	klog.Info("Start registering knode to kosmos control plane...")
	_, err := o.DynamicClient.Resource(util.KnodeGVR).Namespace("").Create(context.TODO(), o.Resource, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(knode) kosmosctl join run warning, create knode failed: %s", err)
	}
	klog.Info("Knode registration successful.")

	return nil
}
