package install

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/version"
)

type CommandInstallOptions struct {
	Namespace      string
	ImageRegistry  string
	Version        string
	Module         string
	HostKubeConfig string

	Client           kubernetes.Interface
	ExtensionsClient extensionsclient.Interface
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
	flags.StringVarP(&o.Namespace, "namespace", "n", util.DefaultNamespace, "Kosmos namespace.")
	flags.StringVarP(&o.ImageRegistry, "private-image-registry", "", util.DefaultImageRepository, "Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.  In addition, you still can use --kube-image-registry to specify the registry for Kubernetes's images.")
	flags.StringVarP(&o.Module, "module", "m", util.DefaultInstallModule, "Kosmos specify the module to install.")
	flags.StringVar(&o.HostKubeConfig, "host-kubeconfig", "", "Absolute path to the host kubeconfig file.")

	return cmd
}

func (o *CommandInstallOptions) Complete(f ctlutil.Factory) error {
	config, err := f.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("kosmosctl install complete error, generate rest config failed: %v", err)
	}

	o.Client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl install complete error, generate basic client failed: %v", err)
	}

	o.ExtensionsClient, err = extensionsclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl install complete error, generate extensions client failed: %v", err)
	}

	return nil
}

func (o *CommandInstallOptions) Validate() error {
	if len(o.Namespace) == 0 {
		return fmt.Errorf("namespace must be specified")
	}

	if o.Module != "clusterlink" && o.HostKubeConfig == "" {
		return fmt.Errorf("host-kubeconfig must be specified")
	}

	return nil
}

func (o *CommandInstallOptions) Run() error {
	switch o.Module {
	case "clusterlink":
		err := o.runClusterlink()
		if err != nil {
			return err
		}
	case "clusterrouter":
		err := o.runClusterrouter()
		if err != nil {
			return err
		}
	case "all":
		err := o.runClusterlink()
		if err != nil {
			return err
		}
		err = o.runClusterrouter()
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *CommandInstallOptions) runClusterlink() error {
	namespace := &corev1.Namespace{}
	namespace.Name = o.Namespace
	_, err := o.Client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, namespace options failed: %v", err)
		}
	}

	clusterlinkServiceAccount, err := util.GenerateServiceAccount(manifest.ClusterlinkNetworkManagerServiceAccount, manifest.ServiceAccountReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.Client.CoreV1().ServiceAccounts(o.Namespace).Create(context.TODO(), clusterlinkServiceAccount, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, serviceaccount options failed: %v", err)
		}
	}

	clusterlinkClusterRole, err := util.GenerateClusterRole(manifest.ClusterlinkNetworkManagerClusterRole, nil)
	if err != nil {
		return err
	}
	_, err = o.Client.RbacV1().ClusterRoles().Create(context.TODO(), clusterlinkClusterRole, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, clusterrole options failed: %v", err)
		}
	}

	clusterlinkClusterRoleBinding, err := util.GenerateClusterRoleBinding(manifest.ClusterlinkNetworkManagerClusterRoleBinding, manifest.ClusterRoleBindingReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.Client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterlinkClusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, clusterrolebinding options failed: %v", err)
		}
	}

	//ToDo CRD manifest parameters are configurable, through obj
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
		_, err = o.ExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), &crds.Items[i], metav1.CreateOptions{})
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("kosmosctl install clusterlink run error, crd options failed: %v", err)
			}
		}
	}

	deployment, err := util.GenerateDeployment(manifest.ClusterlinkNetworkManagerDeployment, manifest.DeploymentReplace{
		Namespace:       o.Namespace,
		ImageRepository: o.ImageRegistry,
		Version:         version.GetReleaseVersion().PatchRelease(),
	})
	if err != nil {
		return err
	}
	_, err = o.Client.AppsV1().Deployments(o.Namespace).Create(context.Background(), deployment, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, deployment options failed: %v", err)
		}
	}

	return nil
}

func (o *CommandInstallOptions) runClusterrouter() error {
	namespace := &corev1.Namespace{}
	namespace.Name = o.Namespace
	_, err := o.Client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterrouter run error, namespace options failed: %v", err)
		}
	}

	hostKubeconfig, err := os.ReadFile(o.HostKubeConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl install clusterrouter run error, host-kubeconfig read failed: %v", err)
	}
	clusterRouterConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host-kubeconfig",
			Namespace: o.Namespace,
		},
		Data: map[string]string{
			"kubeconfig": string(hostKubeconfig),
		},
	}
	_, err = o.Client.CoreV1().ConfigMaps(o.Namespace).Create(context.TODO(), clusterRouterConfigMap, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterrouter run error, configmap options failed: %v", err)
		}
	}

	clusterRouterServiceAccount, err := util.GenerateServiceAccount(manifest.ClusterRouterKnodeServiceAccount, manifest.ServiceAccountReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.Client.CoreV1().ServiceAccounts(o.Namespace).Create(context.TODO(), clusterRouterServiceAccount, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterrouter run error, serviceaccount options failed: %v", err)
		}
	}

	clusterRouterClusterRole, err := util.GenerateClusterRole(manifest.ClusterRouterKnodeClusterRole, nil)
	if err != nil {
		return err
	}
	_, err = o.Client.RbacV1().ClusterRoles().Create(context.TODO(), clusterRouterClusterRole, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterrouter run error, clusterrole options failed: %v", err)
		}
	}

	clusterRouterClusterRoleBinding, err := util.GenerateClusterRoleBinding(manifest.ClusterRouterKnodeClusterRoleBinding, manifest.ClusterRoleBindingReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.Client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRouterClusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterrouter run error, clusterrolebinding options failed: %v", err)
		}
	}

	clusterRouterKnode, err := util.GenerateCustomResourceDefinition(manifest.ClusterRouterKnode, nil)
	if err != nil {
		return err
	}
	_, err = o.ExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), clusterRouterKnode, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterrouter run error, crd options failed: %v", err)
		}
	}

	deployment, err := util.GenerateDeployment(manifest.ClusterRouterKnodeDeployment, manifest.DeploymentReplace{
		Namespace:       o.Namespace,
		ImageRepository: o.ImageRegistry,
		Version:         version.GetReleaseVersion().PatchRelease(),
	})
	if err != nil {
		return err
	}
	_, err = o.Client.AppsV1().Deployments(o.Namespace).Create(context.Background(), deployment, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterrouter run error, deployment options failed: %v", err)
		}
	}

	return nil
}
