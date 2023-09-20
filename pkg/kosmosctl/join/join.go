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
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8syaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/version"
)

var joinExample = templates.Examples(i18n.T(`
		# join member-cluster to master control plane
		kosmosctl join -f member-cluster.yaml --master-kubeconfig=[master-kubeconfig] --cluster-kubeconfig=[member-kubeconfig] 
`))

type CommandJoinOptions struct {
	File              string
	MasterKubeConfig  string
	ClusterKubeConfig string

	DynamicClient *dynamic.DynamicClient
	Client        kubernetes.Interface
}

// NewCmdJoin join this to Clusterlink control plane
func NewCmdJoin(f ctlutil.Factory) *cobra.Command {
	o := &CommandJoinOptions{}

	cmd := &cobra.Command{
		Use:                   "join",
		Short:                 i18n.T("Join this to clusterlink control plane"),
		Long:                  "",
		Example:               joinExample,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(f, cmd, args))
			ctlutil.CheckErr(o.Validate())
			ctlutil.CheckErr(o.Run(f, cmd, args))
			return nil
		},
	}

	cmd.Flags().StringVarP(&o.File, "file", "f", "", "cluster.yaml")
	err := cmd.MarkFlagRequired("file")
	if err != nil {
		fmt.Printf("kosmosctl join cmd error, MarkFlagRequired failed: %s", err)
	}
	cmd.Flags().StringVarP(&o.MasterKubeConfig, "master-kubeconfig", "", "", "master-kubeconfig")
	err = cmd.MarkFlagRequired("master-kubeconfig")
	if err != nil {
		fmt.Printf("kosmosctl join cmd error, MarkFlagRequired failed: %s", err)
	}
	cmd.Flags().StringVarP(&o.ClusterKubeConfig, "cluster-kubeconfig", "", "", "cluster-kubeconfig")
	err = cmd.MarkFlagRequired("cluster-kubeconfig")
	if err != nil {
		fmt.Printf("kosmosctl join cmd error, MarkFlagRequired failed: %s", err)
	}

	return cmd
}

func (o *CommandJoinOptions) Complete(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
	var masterConfig *restclient.Config
	var err error

	masterConfig, err = clientcmd.BuildConfigFromFlags("", o.MasterKubeConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate masterConfig failed: %s", err)
	}

	clusterConfig, err := clientcmd.BuildConfigFromFlags("", o.ClusterKubeConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate memberConfig failed: %s", err)
	}

	o.Client, err = kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate basic client failed: %v", err)
	}

	o.DynamicClient, err = dynamic.NewForConfig(masterConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate dynamic client failed: %s", err)
	}

	return nil
}

func (o *CommandJoinOptions) Validate() error {
	return nil
}

func (o *CommandJoinOptions) Run(f ctlutil.Factory, cmd *cobra.Command, args []string) error {
	// 1. create cluster in master
	cr, err := os.ReadFile(o.File)
	if err != nil {
		return fmt.Errorf("(cluster) kosmosctl join run warning, readfile failed: %s", err)
	}
	decoder := k8syaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err = decoder.Decode(cr, nil, obj)
	if err != nil {
		return fmt.Errorf("(cluster) kosmosctl join run warning, decode failed: %s", err)
	}
	_, err = o.DynamicClient.Resource(schema.GroupVersionResource{
		Group:    "kosmos.io",
		Version:  "v1alpha1",
		Resource: "clusters",
	}).Namespace("").Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("(cluster) kosmosctl join run warning, create cluster failed: %s", err)
	}

	// 2. create namespace in member
	namespace := &corev1.Namespace{}
	namespace.Name = utils.NamespaceClusterLinksystem
	_, err = o.Client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(namespace) kosmosctl join run warning, create namespace failed: %s", err)
	}

	// 3. create secret in member
	masterKubeConfig, err := os.ReadFile(o.MasterKubeConfig)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(secret) kosmosctl join run warning, read masterconfig failed: %s", err)
	}
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.ControlPanelSecretName,
			Namespace: utils.NamespaceClusterLinksystem,
		},
		Data: map[string][]byte{
			"kubeconfig": masterKubeConfig,
		},
	}
	_, err = o.Client.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(secret) kosmosctl join run warning, create secret failed: %s", err)
	}

	// 4. create rbac in member
	clusterRole, err := util.GenerateClusterRole(manifest.ClusterlinkClusterRole, nil)
	if err != nil {
		return fmt.Errorf("(rbac) kosmosctl join run warning, generate clusterrole failed: %s", err)
	}
	_, err = o.Client.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(rbac) kosmosctl join run warning, create clusterrole failed: %s", err)
	}
	clusterRoleBinding, err := util.GenerateClusterRoleBinding(manifest.ClusterlinkClusterRoleBinding, nil)
	if err != nil {
		return fmt.Errorf("(rbac) kosmosctl join run warning, generate clusterrolebinding failed: %s", err)
	}
	_, err = o.Client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(rbac) kosmosctl join run warning, create clusterrolebinding failed: %s", err)
	}

	// 5. create operator in member
	serviceAccount, err := util.GenerateServiceAccount(manifest.ClusterlinkServiceAccount, nil)
	if err != nil {
		return fmt.Errorf("(operator) kosmosctl join run warning, generate serviceaccount failed: %s", err)
	}
	_, err = o.Client.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(operator) kosmosctl join run warning, create serviceaccount failed: %s", err)
	}
	deployment, err := util.GenerateDeployment(manifest.ClusterlinkDeployment, manifest.ClusterlinkDeploymentReplace{
		Version:     version.GetReleaseVersion().PatchRelease(),
		ClusterName: obj.GetName(),
	})
	if err != nil {
		return fmt.Errorf("(operator) kosmosctl join run warning, generate deployment failed: %s", err)
	}
	_, err = o.Client.AppsV1().Deployments(deployment.Namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(operator) kosmosctl join run warning, create deployment failed: %s", err)
	}

	return nil
}
