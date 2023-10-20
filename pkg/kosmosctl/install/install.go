package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/version"
)

var installExample = templates.Examples(i18n.T(`
        # Install all module to Kosmos control plane, e.g: 
        kosmosctl install

		# Install Kosmos control plane, if you need to specify a special master cluster kubeconfig, e.g: 
        kosmosctl install --host-kubeconfig=[host-kubeconfig]

		# Install clusterlink module to Kosmos control plane, e.g: 
        kosmosctl install -m clusterlink

		# Install clustertree module to Kosmos control plane, e.g: 
        kosmosctl install -m clustertree

		# Install coredns module to Kosmos control plane, e.g: 
        kosmosctl install -m coredns
`))

type CommandInstallOptions struct {
	Namespace            string
	ImageRegistry        string
	Version              string
	Module               string
	HostKubeConfig       string
	HostKubeConfigStream []byte
	WaitTime             int

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
		Example:               installExample,
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
	flags.StringVarP(&o.Namespace, "namespace", "n", utils.DefaultNamespace, "Kosmos namespace.")
	flags.StringVarP(&o.ImageRegistry, "private-image-registry", "", utils.DefaultImageRepository, "Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.  In addition, you still can use --kube-image-registry to specify the registry for Kubernetes's images.")
	flags.StringVarP(&o.Module, "module", "m", utils.DefaultInstallModule, "Kosmos specify the module to install.")
	flags.StringVar(&o.HostKubeConfig, "host-kubeconfig", "", "Absolute path to the special host kubeconfig file.")
	flags.IntVarP(&o.WaitTime, "wait-time", "", 120, "Wait the specified time for the Kosmos install ready.")

	return cmd
}

func (o *CommandInstallOptions) Complete(f ctlutil.Factory) error {
	var config *rest.Config
	var err error

	if len(o.HostKubeConfig) > 0 {
		config, err = clientcmd.BuildConfigFromFlags("", o.HostKubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl install complete error, generate host config failed: %s", err)
		}
		o.HostKubeConfigStream, err = os.ReadFile(o.HostKubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl install complete error, read host config failed: %s", err)
		}
	} else {
		config, err = f.ToRESTConfig()
		if err != nil {
			return fmt.Errorf("kosmosctl install complete error, generate rest config failed: %v", err)
		}
		o.HostKubeConfigStream, err = os.ReadFile(filepath.Join(homedir.HomeDir(), ".kube", "config"))
		if err != nil {
			return fmt.Errorf("kosmosctl install complete error, read host config failed: %s", err)
		}
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

	return nil
}

func (o *CommandInstallOptions) Run() error {
	klog.Info("Kosmos starts installing.")
	switch o.Module {
	case "coredns":
		err := o.runCoredns()
		if err != nil {
			return err
		}
		util.CheckInstall("coredns")
	case "clusterlink":
		err := o.runClusterlink()
		if err != nil {
			return err
		}
		util.CheckInstall("Clusterlink")
	case "clustertree":
		err := o.runClustertree()
		if err != nil {
			return err
		}
		util.CheckInstall("Clustertree")
	case "all":
		err := o.runClusterlink()
		if err != nil {
			return err
		}
		err = o.runClustertree()
		if err != nil {
			return err
		}
		util.CheckInstall("Clusterlink && Clustertree")
	}

	return nil
}

func (o *CommandInstallOptions) runClusterlink() error {
	klog.Info("Start creating kosmos-clusterlink...")
	namespace := &corev1.Namespace{}
	namespace.Name = o.Namespace
	_, err := o.Client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, namespace options failed: %v", err)
		}
	}
	klog.Info("Namespace kosmos-system has been created.")

	klog.Info("Start creating kosmos-clusterlink ServiceAccount...")
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
	klog.Info("ServiceAccount clusterlink-network-manager has been created.")

	klog.Info("Start creating kosmos-clusterlink ClusterRole...")
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
	klog.Info("ClusterRole clusterlink-network-manager has been created.")

	klog.Info("Start creating kosmos-clusterlink ClusterRoleBinding...")
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
	klog.Info("ClusterRoleBinding clusterlink-network-manager has been created.")

	klog.Info("Attempting to create clusterlink CRDs...")
	crds := apiextensionsv1.CustomResourceDefinitionList{}
	clusterlinkCluster, err := util.GenerateCustomResourceDefinition(manifest.ClusterlinkCluster, manifest.ClusterlinkReplace{
		Namespace: o.Namespace,
	})
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
		klog.Info("Create CRD " + crds.Items[i].Name + " successful.")
	}

	klog.Info("Start creating kosmos-clusterlink Deployment...")
	clusterlinkDeployment, err := util.GenerateDeployment(manifest.ClusterlinkNetworkManagerDeployment, manifest.DeploymentReplace{
		Namespace:       o.Namespace,
		ImageRepository: o.ImageRegistry,
		Version:         version.GetReleaseVersion().PatchRelease(),
	})
	if err != nil {
		return err
	}
	_, err = o.Client.AppsV1().Deployments(o.Namespace).Create(context.Background(), clusterlinkDeployment, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, deployment options failed: %v", err)
		}
	}
	label := map[string]string{"app": clusterlinkDeployment.Labels["app"]}
	if err = util.WaitPodReady(o.Client, clusterlinkDeployment.Namespace, util.MapToString(label), o.WaitTime); err != nil {
		return fmt.Errorf("kosmosctl install clusterlink run error, deployment options failed: %v", err)
	} else {
		klog.Info("Deployment clusterlink-network-manager has been created.")
	}

	return nil
}

func (o *CommandInstallOptions) runClustertree() error {
	klog.Info("Start creating kosmos-clustertree...")
	namespace := &corev1.Namespace{}
	namespace.Name = o.Namespace
	_, err := o.Client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, namespace options failed: %v", err)
		}
	}
	klog.Info("Namespace kosmos-system has been created.")

	klog.Info("Start creating kosmos-clustertree ServiceAccount...")
	clustertreeServiceAccount, err := util.GenerateServiceAccount(manifest.ClusterTreeKnodeManagerServiceAccount, manifest.ServiceAccountReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.Client.CoreV1().ServiceAccounts(o.Namespace).Create(context.TODO(), clustertreeServiceAccount, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, serviceaccount options failed: %v", err)
		}
	}
	klog.Info("ServiceAccount clustertree-cluster-manager has been created.")

	klog.Info("Start creating kosmos-clustertree ClusterRole...")
	clustertreeClusterRole, err := util.GenerateClusterRole(manifest.ClusterTreeKnodeManagerClusterRole, nil)
	if err != nil {
		return err
	}
	_, err = o.Client.RbacV1().ClusterRoles().Create(context.TODO(), clustertreeClusterRole, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, clusterrole options failed: %v", err)
		}
	}
	klog.Info("ClusterRole clustertree-knode has been created.")

	klog.Info("Start creating kosmos-clustertree ClusterRoleBinding...")
	clustertreeClusterRoleBinding, err := util.GenerateClusterRoleBinding(manifest.ClusterTreeKnodeManagerClusterRoleBinding, manifest.ClusterRoleBindingReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.Client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clustertreeClusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, clusterrolebinding options failed: %v", err)
		}
	}
	klog.Info("ClusterRoleBinding clustertree-knode has been created.")

	klog.Info("Attempting to create kosmos-clustertree knode CRDs...")
	clustertreeKnode, err := util.GenerateCustomResourceDefinition(manifest.ClusterTreeKnode, nil)
	if err != nil {
		return err
	}
	_, err = o.ExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), clustertreeKnode, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, crd options failed: %v", err)
		}
	}
	klog.Info("Create CRD " + clustertreeKnode.Name + " successful.")

	klog.Info("Start creating kosmos-clustertree ConfigMap...")
	clustertreeConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.HostKubeConfigName,
			Namespace: o.Namespace,
		},
		Data: map[string]string{
			"kubeconfig": string(o.HostKubeConfigStream),
		},
	}
	_, err = o.Client.CoreV1().ConfigMaps(o.Namespace).Create(context.TODO(), clustertreeConfigMap, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, configmap options failed: %v", err)
		}
	}
	klog.Info("ConfigMap host-kubeconfig has been created.")

	klog.Info("Start creating kosmos-clustertree Deployment...")
	clustertreeDeployment, err := util.GenerateDeployment(manifest.ClusterTreeKnodeManagerDeployment, manifest.DeploymentReplace{
		Namespace:       o.Namespace,
		ImageRepository: o.ImageRegistry,
		Version:         version.GetReleaseVersion().PatchRelease(),
	})
	if err != nil {
		return err
	}
	_, err = o.Client.AppsV1().Deployments(o.Namespace).Create(context.Background(), clustertreeDeployment, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, deployment options failed: %v", err)
		}
	}
	label := map[string]string{"app": clustertreeDeployment.Labels["app"]}
	if err = util.WaitPodReady(o.Client, clustertreeDeployment.Namespace, util.MapToString(label), o.WaitTime); err != nil {
		return fmt.Errorf("kosmosctl install clustertree run error, deployment options failed: %v", err)
	} else {
		klog.Info("Deployment clustertree-cluster-manager has been created.")
	}

	return nil
}

func (o *CommandInstallOptions) runCoredns() error {
	klog.Info("Start creating kosmos-coredns...")
	namespace := &corev1.Namespace{}
	namespace.Name = o.Namespace
	_, err := o.Client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns run error, namespace options failed: %v", err)
		}
	}
	klog.Infof("Namespace %s has been created.", o.Namespace)

	klog.Info("Start creating kosmos-coredns ServiceAccount...")
	sa, err := util.GenerateServiceAccount(manifest.CorednsServiceAccount, manifest.ServiceAccountReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.Client.CoreV1().ServiceAccounts(o.Namespace).Create(context.TODO(), sa, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns run error, serviceaccount options failed: %v", err)
		}
	}
	klog.Infof("ServiceAccount %s has been created.", sa.Name)

	klog.Info("Start creating kosmos-coredns ClusterRole...")
	cRole, err := util.GenerateClusterRole(manifest.CorednsClusterRole, nil)
	if err != nil {
		return err
	}
	_, err = o.Client.RbacV1().ClusterRoles().Create(context.TODO(), cRole, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns run error, clusterrole options failed: %v", err)
		}
	}
	klog.Infof("ClusterRole %s has been created.", cRole.Name)

	klog.Info("Start creating kosmos-coredns ClusterRoleBinding...")
	crb, err := util.GenerateClusterRoleBinding(manifest.CorednsClusterRoleBinding, manifest.ClusterRoleBindingReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.Client.RbacV1().ClusterRoleBindings().Create(context.TODO(), crb, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns run error, clusterrolebinding options failed: %v", err)
		}
	}
	klog.Infof("ClusterRoleBinding %s has been created.", crb.Name)

	klog.Info("Start creating kosmos-coredns configmaps...")
	coreFile, err := util.GenerateConfigMap(manifest.CorednsCorefile, manifest.ConfigmapReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.Client.CoreV1().ConfigMaps(o.Namespace).Create(context.TODO(), coreFile, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns coreFile run error, configmap options failed: %v", err)
		}
	}
	klog.Info("ConfigMap corefile has been created.")

	customerHosts, err := util.GenerateConfigMap(manifest.CorednsCustomerHosts, manifest.ConfigmapReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.Client.CoreV1().ConfigMaps(o.Namespace).Create(context.TODO(), customerHosts, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns customerHosts run error, configmap options failed: %v", err)
		}
	}
	klog.Info("ConfigMap customerHosts has been created.")

	klog.Info("Attempting to create coredns CRDs, coredns reuses clusterlink's cluster CRD")
	crd, err := util.GenerateCustomResourceDefinition(manifest.ClusterlinkCluster, manifest.ClusterlinkReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.ExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), crd, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns run error, crd options failed: %v", err)
		}
	}
	klog.Infof("Create CRD %s successful.", crd.Name)

	klog.Info("Start creating kosmos-coredns Deployment...")
	deploy, err := util.GenerateDeployment(manifest.CorednsDeployment, manifest.DeploymentReplace{
		Namespace:       o.Namespace,
		ImageRepository: o.ImageRegistry,
	})
	if err != nil {
		return err
	}
	_, err = o.Client.AppsV1().Deployments(o.Namespace).Create(context.Background(), deploy, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns run error, deployment options failed: %v", err)
		}
	}
	if err = util.WaitDeploymentReady(o.Client, deploy, o.WaitTime); err != nil {
		return fmt.Errorf("kosmosctl install coredns run error, deployment options failed: %v", err)
	} else {
		klog.Info("Deployment coredns has been created.")
	}

	klog.Info("Attempting to create coredns service...")
	svc, err := util.GenerateService(manifest.CorednsService, manifest.ServiceReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.Client.CoreV1().Services(o.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns run error, service options failed: %v", err)
		}
	}
	klog.Infof("Create service %s successful.", svc.Name)

	return nil
}
