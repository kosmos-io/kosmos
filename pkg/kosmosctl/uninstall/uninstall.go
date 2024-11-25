package uninstall

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	extensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/version"
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

		# Uninstall kosmos-scheduler from Kosmos control plane, e.g: 
		kosmosctl uninstall -m scheduler

		# Uninstall coredns module from Kosmos control plane, e.g: 
        kosmosctl uninstall -m coredns
`))

type CommandUninstallOptions struct {
	Namespace  string
	Module     string
	KubeConfig string
	Context    string

	KosmosClient        versioned.Interface
	K8sClient           kubernetes.Interface
	K8sDynamicClient    *dynamic.DynamicClient
	K8sExtensionsClient extensionsclient.Interface

	Version string
}

// NewCmdUninstall Uninstall the Kosmos control plane in a Kubernetes cluster.
func NewCmdUninstall() *cobra.Command {
	o := &CommandUninstallOptions{}

	cmd := &cobra.Command{
		Use:                   "uninstall",
		Short:                 i18n.T("Uninstall the Kosmos control plane in a Kubernetes cluster"),
		Long:                  "",
		Example:               uninstallExample,
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
	flags.StringVarP(&o.Namespace, "namespace", "n", utils.DefaultNamespace, "Kosmos namespace.")
	flags.StringVarP(&o.Module, "module", "m", utils.All, "Kosmos specify the module to uninstall.")
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Absolute path to the special kubeconfig file.")
	flags.StringVar(&o.Context, "context", "", "The name of the kubeconfig context.")
	flags.StringVar(&o.Version, "version", "", "image version for uninstall images")
	return cmd
}

func (o *CommandUninstallOptions) Complete() error {
	config, err := utils.RestConfig(o.KubeConfig, o.Context)
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall complete error, generate config failed: %s", err)
	}

	if o.Version == "" {
		o.Version = fmt.Sprintf("v%s", version.GetReleaseVersion().PatchRelease())
	}

	o.KosmosClient, err = versioned.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall complete error, generate Kosmos client failed: %v", err)
	}

	o.K8sClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl uninstall complete error, generate basic client failed: %v", err)
	}

	o.K8sDynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate dynamic client failed: %s", err)
	}

	o.K8sExtensionsClient, err = extensionsclient.NewForConfig(config)
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
	case "scheduler":
		err := o.runScheduler()
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
		err = o.K8sClient.CoreV1().Namespaces().Delete(context.TODO(), o.Namespace, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall all module run error, namespace options failed: %v", err)
		}
	}

	return nil
}

func (o *CommandUninstallOptions) runClusterlink() error {
	klog.Info("Start uninstalling clusterlink from kosmos control plane...")
	clusterlinkDeploy, err := util.GenerateDeployment(manifest.ClusterlinkNetworkManagerDeployment, manifest.DeploymentReplace{
		Namespace: o.Namespace,
		Version:   o.Version,
	})
	if err != nil {
		return err
	}
	err = o.K8sClient.AppsV1().Deployments(o.Namespace).Delete(context.Background(), clusterlinkDeploy.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, deployment options failed: %v", err)
		}
	} else {
		klog.Info("Deployment " + clusterlinkDeploy.Name + " is deleted.")
	}

	clusters, err := o.KosmosClient.KosmosV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, list cluster failed: %v", err)
		}
	} else if clusters != nil && len(clusters.Items) > 0 {
		klog.Info("kosmosctl uninstall warning, skip removing cluster crd because cr instance exists")
	} else {
		clusterCRD, err := util.GenerateCustomResourceDefinition(manifest.Cluster, nil)
		if err != nil {
			return err
		}
		err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), clusterCRD.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, cluster crd delete failed: %v", err)
		}
		klog.Info("CRD " + clusterCRD.Name + " is deleted.")
	}

	clusternodes, err := o.KosmosClient.KosmosV1alpha1().ClusterNodes().List(context.TODO(), metav1.ListOptions{})
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
		err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), clusternodeCRD.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, clusternode crd delete failed: %v", err)
		}
		klog.Info("CRD " + clusternodeCRD.Name + " is deleted.")
	}

	nodeconfigs, err := o.KosmosClient.KosmosV1alpha1().NodeConfigs().List(context.TODO(), metav1.ListOptions{})
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
		err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), nodeConfigCRD.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, clusternode crd delete failed: %v", err)
		}
		klog.Info("CRD " + nodeConfigCRD.Name + " is deleted.")
	}

	clusterlinkCRB, err := util.GenerateClusterRoleBinding(manifest.ClusterlinkNetworkManagerClusterRoleBinding, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), clusterlinkCRB.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, clusterrolebinding options failed: %v", err)
		}
	} else {
		klog.Info("ClusterRoleBinding " + clusterlinkCRB.Name + " is deleted.")
	}

	clusterlinkCR, err := util.GenerateClusterRole(manifest.ClusterlinkNetworkManagerClusterRole, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.RbacV1().ClusterRoles().Delete(context.TODO(), clusterlinkCR.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, clusterrole options failed: %v", err)
		}
	} else {
		klog.Info("ClusterRole " + clusterlinkCR.Name + " is deleted.")
	}

	clusterlinkSA, err := util.GenerateServiceAccount(manifest.ClusterlinkNetworkManagerServiceAccount, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.CoreV1().ServiceAccounts(o.Namespace).Delete(context.TODO(), clusterlinkSA.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clusterlink run error, serviceaccount options failed: %v", err)
		}
	} else {
		klog.Info("ServiceAccount " + clusterlinkSA.Name + " is deleted.")
	}

	clustertreeDeploy, err := util.GenerateDeployment(manifest.ClusterTreeClusterManagerDeployment, nil)
	if err != nil {
		return err
	}
	_, err = o.K8sClient.AppsV1().Deployments(o.Namespace).Get(context.Background(), clustertreeDeploy.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			operatorDeploy, err := util.GenerateDeployment(manifest.ClusterlinkOperatorDeployment, manifest.DeploymentReplace{
				Namespace: o.Namespace,
				Version:   o.Version,
			})
			if err != nil {
				return fmt.Errorf("kosmosctl uninstall clusterlink run error, generate operator deployment failed: %s", err)
			}
			err = o.K8sClient.AppsV1().Deployments(o.Namespace).Delete(context.Background(), operatorDeploy.Name, metav1.DeleteOptions{})
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("kosmosctl uninstall clusterlink run error, operator deployment options failed: %v", err)
				}
			} else {
				klog.Info("Deployment " + operatorDeploy.Name + " is deleted.")
			}
		}
	} else {
		klog.Info("Kosmos-Clustertree is still running, skip uninstall Clusterlink-Operator.")
	}

	klog.Info("Clusterlink uninstalled.")
	return nil
}

func (o *CommandUninstallOptions) runClustertree() error {
	klog.Info("Start uninstalling clustertree from kosmos control plane...")
	clustertreeDeploy, err := util.GenerateDeployment(manifest.ClusterTreeClusterManagerDeployment, manifest.DeploymentReplace{
		Namespace: o.Namespace,
		Version:   o.Version,
	})
	if err != nil {
		return err
	}
	err = o.K8sClient.AppsV1().Deployments(o.Namespace).Delete(context.Background(), clustertreeDeploy.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, deployment options failed: %v", err)
		}
	} else {
		klog.Info("Deployment " + clustertreeDeploy.Name + " is deleted.")
		clustertreeSecret, err := util.GenerateSecret(manifest.ClusterTreeClusterManagerSecret, manifest.SecretReplace{
			Namespace: o.Namespace,
			Cert:      "",
			Key:       "",
		})
		if err != nil {
			return err
		}
		err = o.K8sClient.CoreV1().Secrets(o.Namespace).Delete(context.Background(), clustertreeSecret.Name, metav1.DeleteOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("kosmosctl uninstall clustertree secret run error, secret options failed: %v", err)
			}
		} else {
			klog.Info("Secret " + clustertreeSecret.Name + " is deleted.")
		}
	}

	clusters, err := o.KosmosClient.KosmosV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, list cluster failed: %v", err)
		}
	} else if clusters != nil && len(clusters.Items) > 0 {
		klog.Info("kosmosctl uninstall warning, skip removing cluster crd because cr instance exists")
	} else {
		clusterCRD, err := util.GenerateCustomResourceDefinition(manifest.Cluster, nil)
		if err != nil {
			return err
		}
		err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), clusterCRD.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, cluster crd delete failed: %v", err)
		}
		klog.Info("CRD " + clusterCRD.Name + " is deleted.")
	}

	err = o.K8sClient.CoreV1().ConfigMaps(o.Namespace).Delete(context.TODO(), utils.HostKubeConfigName, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, configmap options failed: %v", err)
		}
	} else {
		klog.Info("ConfigMap " + utils.HostKubeConfigName + " is deleted.")
	}

	clustertreeCRB, err := util.GenerateClusterRoleBinding(manifest.ClusterTreeClusterRoleBinding, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), clustertreeCRB.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, clusterrolebinding options failed: %v", err)
		}
	} else {
		klog.Info("ClusterRoleBinding " + clustertreeCRB.Name + " is deleted.")
	}

	clustertreeCR, err := util.GenerateClusterRole(manifest.ClusterTreeClusterRole, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.RbacV1().ClusterRoles().Delete(context.TODO(), clustertreeCR.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, clusterrole options failed: %v", err)
		}
	} else {
		klog.Info("ClusterRole " + clustertreeCR.Name + " is deleted.")
	}

	clustertreeSA, err := util.GenerateServiceAccount(manifest.ClusterTreeServiceAccount, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.CoreV1().ServiceAccounts(o.Namespace).Delete(context.TODO(), clustertreeSA.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall clustertree run error, serviceaccount options failed: %v", err)
		}
	} else {
		klog.Info("ServiceAccount " + clustertreeSA.Name + " is deleted.")
	}

	clusterlinkDeploy, err := util.GenerateDeployment(manifest.ClusterlinkNetworkManagerDeployment, nil)
	if err != nil {
		return err
	}
	_, err = o.K8sClient.AppsV1().Deployments(o.Namespace).Get(context.Background(), clusterlinkDeploy.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			operatorDeploy, err := util.GenerateDeployment(manifest.ClusterlinkOperatorDeployment, manifest.DeploymentReplace{
				Namespace: o.Namespace,
				Version:   o.Version,
			})
			if err != nil {
				return fmt.Errorf("kosmosctl uninstall clustertree run error, generate operator deployment failed: %s", err)
			}
			err = o.K8sClient.AppsV1().Deployments(o.Namespace).Delete(context.Background(), operatorDeploy.Name, metav1.DeleteOptions{})
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("kosmosctl uninstall clustertree run error, operator deployment options failed: %v", err)
				}
			} else {
				klog.Info("Deployment " + operatorDeploy.Name + " is deleted.")
			}
		}
	} else {
		klog.Info("Kosmos-Clusterlink is still running, skip uninstall Clusterlink-Operator.")
	}

	klog.Info("Clustertree uninstalled.")
	return nil
}

func (o *CommandUninstallOptions) runScheduler() error {
	klog.Info("Start uninstalling scheduler from kosmos control plane...")
	schedulerDeploy, err := util.GenerateDeployment(manifest.SchedulerDeployment, manifest.DeploymentReplace{
		Namespace: o.Namespace,
		Version:   o.Version,
	})
	if err != nil {
		return err
	}
	err = o.K8sClient.AppsV1().Deployments(o.Namespace).Delete(context.Background(), schedulerDeploy.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall scheduler run error, deployment options failed: %v", err)
		}
	}

	schedulerConfig, err := util.GenerateConfigMap(manifest.SchedulerConfigmap, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.CoreV1().ConfigMaps(o.Namespace).Delete(context.TODO(), schedulerConfig.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall scheduler run error, configmap options failed: %v", err)
	}
	klog.Info("Configmap " + schedulerConfig.Name + " is deleted.")

	schedulerCRB, err := util.GenerateClusterRoleBinding(manifest.SchedulerClusterRoleBinding, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), schedulerCRB.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall scheduler run error, clusterrolebinding options failed: %v", err)
		}
	} else {
		klog.Info("ClusterRoleBinding " + schedulerCRB.Name + " is deleted.")
	}

	schedulerCR, err := util.GenerateClusterRole(manifest.SchedulerClusterRole, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.RbacV1().ClusterRoles().Delete(context.TODO(), schedulerCR.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall scheduler run error, clusterrole options failed: %v", err)
		}
	} else {
		klog.Info("ClusterRole " + schedulerCR.Name + " is deleted.")
	}

	schedulerSA, err := util.GenerateServiceAccount(manifest.SchedulerServiceAccount, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.CoreV1().ServiceAccounts(o.Namespace).Delete(context.TODO(), schedulerSA.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall scheduler run error, serviceaccount options failed: %v", err)
		}
	} else {
		klog.Info("ServiceAccount " + schedulerSA.Name + " is deleted.")
	}

	clusterDPs, err := o.KosmosClient.KosmosV1alpha1().ClusterDistributionPolicies().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall kosmos-scheduler run error, list cluster distribution policy failed: %v", err)
		}
	} else if clusterDPs != nil && len(clusterDPs.Items) > 0 {
		klog.Info("kosmosctl uninstall warning, skip removing cluster distribution policy crd because cr instance exists")
	} else {
		schedulerCDP, err := util.GenerateCustomResourceDefinition(manifest.SchedulerClusterDistributionPolicies, nil)
		if err != nil {
			return err
		}
		err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), schedulerCDP.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall kosmos-scheduler run error, cluster distribution policy crd delete failed: %v", err)
		}
		klog.Info("CRD " + schedulerCDP.Name + " is deleted.")
	}

	// ToDo delete namespace level crd DistributionPolicy

	klog.Info("Scheduler uninstalled.")
	return nil
}

func (o *CommandUninstallOptions) runCoredns() error {
	klog.Info("Start uninstalling coredns ...")
	deploy, err := util.GenerateDeployment(manifest.CorednsDeployment, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.AppsV1().Deployments(o.Namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, deployment options failed: %v", err)
	}
	klog.Infof("Deployment %s is deleted.", deploy.Name)

	svc, err := util.GenerateService(manifest.CorednsService, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.CoreV1().Services(o.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, service options failed: %v", err)
	}
	klog.Info("Service " + svc.Name + " is deleted.")

	coreFile, err := util.GenerateConfigMap(manifest.CorednsCorefile, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.CoreV1().ConfigMaps(o.Namespace).Delete(context.TODO(), coreFile.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, configmap options failed: %v", err)
	}
	klog.Info("Configmap " + svc.Name + " is deleted.")

	customerHosts, err := util.GenerateConfigMap(manifest.CorednsCustomerHosts, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.CoreV1().ConfigMaps(o.Namespace).Delete(context.TODO(), customerHosts.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, configmap options failed: %v", err)
	}
	klog.Info("Configmap " + svc.Name + " is deleted.")

	clusters, err := o.K8sDynamicClient.Resource(util.ClusterGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, list cluster failed: %v", err)
	} else if len(clusters.Items) > 0 {
		klog.Info("kosmosctl uninstall warning, skip removing cluster crd because cr instance exists")
	} else {
		clusterCRD, _ := util.GenerateCustomResourceDefinition(manifest.Cluster, nil)
		if err != nil {
			return err
		}
		err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), clusterCRD.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl uninstall coredns run error, cluster crd delete failed: %v", err)
		}
		klog.Infof("CRD %s is deleted.", clusterCRD.Name)
	}

	crb, err := util.GenerateClusterRoleBinding(manifest.CorednsClusterRoleBinding, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), crb.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, clusterrolebinding options failed: %v", err)
	}
	klog.Info("ClusterRoleBinding " + crb.Name + " is deleted.")

	cRole, err := util.GenerateClusterRole(manifest.CorednsClusterRole, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.RbacV1().ClusterRoles().Delete(context.TODO(), cRole.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, clusterrole options failed: %v", err)
	}
	klog.Info("ClusterRole " + cRole.Name + " is deleted.")

	sa, err := util.GenerateServiceAccount(manifest.CorednsServiceAccount, nil)
	if err != nil {
		return err
	}
	err = o.K8sClient.CoreV1().ServiceAccounts(o.Namespace).Delete(context.TODO(), sa.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl uninstall coredns run error, serviceaccount options failed: %v", err)
	}
	klog.Info("ServiceAccount " + sa.Name + " is deleted.")

	klog.Info("Coredns was uninstalled.")
	return nil
}
