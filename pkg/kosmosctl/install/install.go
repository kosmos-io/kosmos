package install

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsclient "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacclient "k8s.io/client-go/kubernetes/typed/rbac/v1"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/version"
)

const (
	clusterlinkSystem         = "clusterlink-system"
	clusterlinkNetworkManager = "clusterlink-network-manager"
)

type CommandInstallOptions struct {
	Namespace     string
	ImageRegistry string
	Version       string

	AppsClient          *appsclient.AppsV1Client
	CoreClient          *coreclient.CoreV1Client
	RbacClient          *rbacclient.RbacV1Client
	ExtensionsClientSet extensionsclientset.Interface
}

// NewCmdInstall Install the Kosmos control plane in a Kubernetes cluster.
func NewCmdInstall(f ctlutil.Factory) *cobra.Command {
	o := &CommandInstallOptions{}

	cmd := &cobra.Command{
		Use:                   "install",
		Short:                 i18n.T("Install the Kosmos control plane in a Kubernetes cluster"),
		Long:                  "",
		Example:               "",
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
	flags.StringVarP(&o.Namespace, "namespace", "n", clusterlinkSystem, "Kubernetes namespace.")
	flags.StringVarP(&o.ImageRegistry, "private-image-registry", "", "ghcr.io/kosmos-io", "Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.  In addition, you still can use --kube-image-registry to specify the registry for Kubernetes's images.")

	return cmd
}

func (o *CommandInstallOptions) Complete(f ctlutil.Factory) error {
	config, err := f.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("kosmosctl install complete error, generate rest config failed: %v", err)
	}

	o.AppsClient, err = appsclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl install complete error, generate apps client failed: %v", err)
	}

	o.CoreClient, err = coreclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl install complete error, generate core client failed: %v", err)
	}

	o.RbacClient, err = rbacclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl install complete error, generate rbac client failed: %v", err)
	}

	o.ExtensionsClientSet, err = extensionsclientset.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl install complete error, generate extensions client failed: %v", err)
	}

	return nil
}

func (o *CommandInstallOptions) Validate() error {
	if len(o.Namespace) == 0 {
		return fmt.Errorf("namespace must be specified")
	}

	return nil
}

func (o *CommandInstallOptions) Run() error {
	namespace := &corev1.Namespace{}
	namespace.Name = o.Namespace
	_, err := o.CoreClient.Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install run warning, namespace options failed: %v", err)
		}
		return fmt.Errorf("kosmosctl install run error, namespace options failed: %v", err)
	}

	serviceAccount := &corev1.ServiceAccount{}
	serviceAccount.Name = clusterlinkNetworkManager
	_, err = o.CoreClient.ServiceAccounts(o.Namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install run error, serviceaccount options failed: %v", err)
		}
	}

	clusterRole := &rbacv1.ClusterRole{}
	clusterRole.Name = clusterlinkNetworkManager
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			Verbs:     []string{"*"},
			Resources: []string{"*"},
			APIGroups: []string{"*"},
		},
		{
			Verbs:           []string{"get"},
			NonResourceURLs: []string{"*"},
		},
	}
	_, err = o.RbacClient.ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install run error, clusterrole options failed: %v", err)
		}
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	clusterRoleBinding.Name = clusterlinkNetworkManager
	clusterRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     clusterlinkNetworkManager,
	}
	clusterRoleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      clusterlinkNetworkManager,
			Namespace: o.Namespace,
		},
	}
	_, err = o.RbacClient.ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install run error, clusterrolebinding options failed: %v", err)
		}
	}

	//ToDo Manifest parameters are configurable, through obj
	crds := apiextensionsv1.CustomResourceDefinitionList{}
	clusterlinkCluster, err := util.GenerateCustomResourceDefinition(manifest.ClusterlinkCluster, nil)
	if err != nil {
		return err
	}
	clusterlinkClusterNode, err := util.GenerateCustomResourceDefinition(manifest.ClusterlinkClusterNode, nil)
	if err != nil {
		return err
	}
	clusterlinkNodeConfig, err := util.GenerateCustomResourceDefinition(manifest.ClusterlinkNodeConfig, nil)
	if err != nil {
		return err
	}
	crds.Items = append(crds.Items, *clusterlinkCluster, *clusterlinkClusterNode, *clusterlinkNodeConfig)
	for i := range crds.Items {
		_, err = o.ExtensionsClientSet.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), &crds.Items[i], metav1.CreateOptions{})
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("kosmosctl install run error, crd options failed: %v", err)
			}
		}
	}

	deployment, err := util.GenerateDeployment(manifest.ClusterlinkNetworkManagerDeployment, manifest.DeploymentReplace{
		Namespace: o.Namespace,
		Image:     o.ImageRegistry,
		Version:   version.GetReleaseVersion().PatchRelease(),
	})
	if err != nil {
		return err
	}
	_, err = o.AppsClient.Deployments(o.Namespace).Create(context.Background(), deployment, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install run error, deployment options failed: %v", err)
		}
	}

	return nil
}
