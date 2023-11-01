package uninstall

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	extensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

var uninstallExample = templates.Examples(i18n.T(`
        # Uninstall all module from Kosmos control plane, e.g: 
        kosmosctl uninstall

		# Uninstall Kosmos control plane, if you need to specify a special control plane cluster kubeconfig, e.g: 
        kosmosctl uninstall --kubeconfig ~/kubeconfig/cluster-kubeconfig

		# Uninstall clusterlink module from Kosmos control plane, e.g: 
        kosmosctl uninstall -m clusterlink

		# Uninstall clustertree module from Kosmos control plane, e.g: 
        kosmosctl uninstall -m clustertree

		# Uninstall coredns module from Kosmos control plane, e.g: 
        kosmosctl uninstall -m coredns
`))

type CommandUninstallOptions struct {
	Namespace  string
	Module     string
	KubeConfig string

	Client           kubernetes.Interface
	DynamicClient    *dynamic.DynamicClient
	ExtensionsClient extensionsclient.Interface
}

// NewCmdUninstall Uninstall the Kosmos control plane in a Kubernetes cluster.
func NewCmdUninstall(f ctlutil.Factory) *cobra.Command {
	o := &CommandUninstallOptions{}

	cmd := &cobra.Command{
		Use:                   "uninstall",
		Short:                 i18n.T("Uninstall the Kosmos control plane in a Kubernetes cluster"),
		Long:                  "",
		Example:               uninstallExample,
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
	flags.StringVarP(&o.Module, "module", "m", utils.DefaultInstallModule, "Kosmos specify the module to uninstall.")
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Absolute path to the special kubeconfig file.")

	return cmd
}

func (o *CommandUninstallOptions) Complete(f ctlutil.Factory) error {
	var config *rest.Config
	var err error

	if len(o.KubeConfig) > 0 {
		config, err = clientcmd.BuildConfigFromFlags("", o.KubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl uninstall complete error, generate config failed: %s", err)
		}
	} else {
		config, err = f.ToRESTConfig()
		if err != nil {
			return fmt.Errorf("kosmosctl uninstall complete error, generate rest config failed: %v", err)
		}
	}

	o.Client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall complete error, generate basic client failed: %v", err)
	}

	o.DynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate dynamic client failed: %s", err)
	}

	o.ExtensionsClient, err = extensionsclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall complete error, generate extensions client failed: %v", err)
	}

	return nil
}

func (o *CommandUninstallOptions) Validate() error {
	if len(o.Namespace) == 0 {
		return fmt.Errorf("kosmosctl uninstall validate error, namespace must be specified")
	}

	return nil
}

func (o *CommandUninstallOptions) Run() error {
	klog.Info("Kosmos starts uninstalling.")
	switch o.Module {
	case "coredns":
		err := o.runCoredns()
		if err != nil {
			return err
		}
	case "clusterlink":
		err := o.runClusterlink()
		if err != nil {
			return err
		}
	case "clustertree":
		err := o.runClustertree()
		if err != nil {
			return err
		}
	case "all":
		err := o.runClusterlink()
		if err != nil {
			return err
		}
		err = o.runClustertree()
		if err != nil {
			return err
		}
		err = o.Client.CoreV1().Namespaces().Delete(context.TODO(), o.Namespace, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall all module run error, namespace options failed: %v", err)
		}
	}

	return nil
}

func (o *CommandUninstallOptions) runClusterlink() error {
	klog.Info("Start uninstalling clusterlink from kosmos control plane...")
	clusterlinkDeployment, err := util.GenerateDeployment(manifest.ClusterlinkNetworkManagerDeployment, nil)
	if err != nil {
		return err
	}
	err = o.Client.AppsV1().Deployments(o.Namespace).Delete(context.Background(), clusterlinkDeployment.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, deployment options failed: %v", err)
		}
	} else {
		klog.Info("Deployment " + clusterlinkDeployment.Name + " is deleted.")
	}

	var clusters, clusternodes, nodeconfigs *unstructured.UnstructuredList
	clusters, err = o.DynamicClient.Resource(util.ClusterGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, list cluster failed: %v", err)
		}
	} else if clusters != nil && len(clusters.Items) > 0 {
		klog.Info("kosmosctl uninstall warning, skip removing cluster crd because cr instance exists")
	} else {
		clusterCRD, err := util.GenerateCustomResourceDefinition(manifest.ClusterlinkCluster, nil)
		if err != nil {
			return err
		}
		err = o.ExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), clusterCRD.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, cluster crd delete failed: %v", err)
		}
		klog.Info("CRD " + clusterCRD.Name + " is deleted.")
	}

	clusternodes, err = o.DynamicClient.Resource(util.ClusterNodeGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, list clusternode failed: %v", err)
		}
	} else if clusternodes != nil && len(clusternodes.Items) > 0 {
		klog.Info("kosmosctl uninstall warning, skip removing clusternode crd because cr instance exists")
	} else {
		clusternodeCRD, err := util.GenerateCustomResourceDefinition(manifest.ClusterlinkClusterNode, nil)
		if err != nil {
			return err
		}
		err = o.ExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), clusternodeCRD.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, clusternode crd delete failed: %v", err)
		}
		klog.Info("CRD " + clusternodeCRD.Name + " is deleted.")
	}

	nodeconfigs, err = o.DynamicClient.Resource(util.NodeConfigGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, list nodeconfig failed: %v", err)
		}
	} else if nodeconfigs != nil && len(nodeconfigs.Items) > 0 {
		klog.Info("kosmosctl uninstall warning, skip removing nodeconfig crd because cr instance exists")
	} else {
		nodeConfigCRD, err := util.GenerateCustomResourceDefinition(manifest.ClusterlinkNodeConfig, nil)
		if err != nil {
			return err
		}
		err = o.ExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), nodeConfigCRD.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, clusternode crd delete failed: %v", err)
		}
		klog.Info("CRD " + nodeConfigCRD.Name + " is deleted.")
	}

	clusterlinkClusterRoleBinding, err := util.GenerateClusterRoleBinding(manifest.ClusterlinkNetworkManagerClusterRoleBinding, nil)
	if err != nil {
		return err
	}
	err = o.Client.RbacV1().ClusterRoleBindings().Delete(context.TODO(), clusterlinkClusterRoleBinding.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, clusterrolebinding options failed: %v", err)
		}
	} else {
		klog.Info("ClusterRoleBinding " + clusterlinkClusterRoleBinding.Name + " is deleted.")
	}

	clusterlinkClusterRole, err := util.GenerateClusterRole(manifest.ClusterlinkNetworkManagerClusterRole, nil)
	if err != nil {
		return err
	}
	err = o.Client.RbacV1().ClusterRoles().Delete(context.TODO(), clusterlinkClusterRole.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, clusterrole options failed: %v", err)
		}
	} else {
		klog.Info("ClusterRole " + clusterlinkClusterRole.Name + " is deleted.")
	}

	clusterlinkServiceAccount, err := util.GenerateServiceAccount(manifest.ClusterlinkNetworkManagerServiceAccount, nil)
	if err != nil {
		return err
	}
	err = o.Client.CoreV1().ServiceAccounts(o.Namespace).Delete(context.TODO(), clusterlinkServiceAccount.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, serviceaccount options failed: %v", err)
		}
	} else {
		klog.Info("ServiceAccount " + clusterlinkServiceAccount.Name + " is deleted.")
	}

	klog.Info("Clusterlink uninstalled.")
	return nil
}

func (o *CommandUninstallOptions) runClustertree() error {
	klog.Info("Start uninstalling clustertree from kosmos control plane...")
	clustertreeDeployment, err := util.GenerateDeployment(manifest.ClusterTreeKnodeManagerDeployment, nil)
	if err != nil {
		return err
	}
	err = o.Client.AppsV1().Deployments(o.Namespace).Delete(context.Background(), clustertreeDeployment.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, deployment options failed: %v", err)
		}
	} else {
		klog.Info("Deployment " + clustertreeDeployment.Name + " is deleted.")
	}

	var knodes *unstructured.UnstructuredList
	knodes, err = o.DynamicClient.Resource(util.KnodeGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, list knode failed: %v", err)
		}
	} else if knodes != nil && len(knodes.Items) > 0 {
		klog.Info("kosmosctl uninstall warning, skip removing knode crd because cr instance exists")
	} else {
		knodeCRD, err := util.GenerateCustomResourceDefinition(manifest.ClusterTreeKnode, nil)
		if err != nil {
			return err
		}
		err = o.ExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), knodeCRD.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, knode crd delete failed: %v", err)
		}
		klog.Info("CRD " + knodeCRD.Name + " is deleted.")
	}

	err = o.Client.CoreV1().ConfigMaps(utils.DefaultNamespace).Delete(context.TODO(), utils.HostKubeConfigName, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, configmap options failed: %v", err)
		}
	} else {
		klog.Info("ConfigMap " + utils.HostKubeConfigName + " is deleted.")
	}

	clustertreeClusterRoleBinding, err := util.GenerateClusterRoleBinding(manifest.ClusterTreeKnodeManagerClusterRoleBinding, nil)
	if err != nil {
		return err
	}
	err = o.Client.RbacV1().ClusterRoleBindings().Delete(context.TODO(), clustertreeClusterRoleBinding.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, clusterrolebinding options failed: %v", err)
		}
	} else {
		klog.Info("ClusterRoleBinding " + clustertreeClusterRoleBinding.Name + " is deleted.")
	}

	clustertreeClusterRole, err := util.GenerateClusterRole(manifest.ClusterTreeKnodeManagerClusterRole, nil)
	if err != nil {
		return err
	}
	err = o.Client.RbacV1().ClusterRoles().Delete(context.TODO(), clustertreeClusterRole.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, clusterrole options failed: %v", err)
		}
	} else {
		klog.Info("ClusterRole " + clustertreeClusterRole.Name + " is deleted.")
	}

	clustertreeServiceAccount, err := util.GenerateServiceAccount(manifest.ClusterTreeKnodeManagerServiceAccount, nil)
	if err != nil {
		return err
	}
	err = o.Client.CoreV1().ServiceAccounts(o.Namespace).Delete(context.TODO(), clustertreeServiceAccount.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, serviceaccount options failed: %v", err)
		}
	} else {
		klog.Info("ServiceAccount " + clustertreeServiceAccount.Name + " is deleted.")
	}

	klog.Info("Clustertree uninstalled.")
	return nil
}

func (o *CommandUninstallOptions) runCoredns() error {
	klog.Info("Start uninstalling coredns ...")
	deploy, err := util.GenerateDeployment(manifest.CorednsDeployment, nil)
	if err != nil {
		return err
	}
	err = o.Client.AppsV1().Deployments(o.Namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, deployment options failed: %v", err)
	}
	klog.Infof("Deployment %s is deleted.", deploy.Name)

	svc, err := util.GenerateService(manifest.CorednsService, nil)
	if err != nil {
		return err
	}
	err = o.Client.CoreV1().Services(o.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, service options failed: %v", err)
	}
	klog.Info("Service " + svc.Name + " is deleted.")

	coreFile, err := util.GenerateConfigMap(manifest.CorednsCorefile, nil)
	if err != nil {
		return err
	}
	err = o.Client.CoreV1().ConfigMaps(o.Namespace).Delete(context.TODO(), coreFile.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, configmap options failed: %v", err)
	}
	klog.Info("Configmap " + svc.Name + " is deleted.")

	customerHosts, err := util.GenerateConfigMap(manifest.CorednsCustomerHosts, nil)
	if err != nil {
		return err
	}
	err = o.Client.CoreV1().ConfigMaps(o.Namespace).Delete(context.TODO(), customerHosts.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, configmap options failed: %v", err)
	}
	klog.Info("Configmap " + svc.Name + " is deleted.")

	clusters, err := o.DynamicClient.Resource(util.ClusterGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, list cluster failed: %v", err)
	} else if len(clusters.Items) > 0 {
		klog.Info("kosmosctl uninstall warning, skip removing cluster crd because cr instance exists")
	} else {
		clusterCRD, _ := util.GenerateCustomResourceDefinition(manifest.ClusterlinkCluster, nil)
		if err != nil {
			return err
		}
		err = o.ExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), clusterCRD.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall coredns run error, cluster crd delete failed: %v", err)
		}
		klog.Infof("CRD %s is deleted.", clusterCRD.Name)
	}

	crb, err := util.GenerateClusterRoleBinding(manifest.CorednsClusterRoleBinding, nil)
	if err != nil {
		return err
	}
	err = o.Client.RbacV1().ClusterRoleBindings().Delete(context.TODO(), crb.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, clusterrolebinding options failed: %v", err)
	}
	klog.Info("ClusterRoleBinding " + crb.Name + " is deleted.")

	cRole, err := util.GenerateClusterRole(manifest.CorednsClusterRole, nil)
	if err != nil {
		return err
	}
	err = o.Client.RbacV1().ClusterRoles().Delete(context.TODO(), cRole.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl install coredns run error, clusterrole options failed: %v", err)
	}
	klog.Info("ClusterRole " + cRole.Name + " is deleted.")

	sa, err := util.GenerateServiceAccount(manifest.CorednsServiceAccount, nil)
	if err != nil {
		return err
	}
	err = o.Client.CoreV1().ServiceAccounts(o.Namespace).Delete(context.TODO(), sa.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, serviceaccount options failed: %v", err)
	}
	klog.Info("ServiceAccount " + sa.Name + " is deleted.")

	klog.Info("Coredns was uninstalled.")
	return nil
}
