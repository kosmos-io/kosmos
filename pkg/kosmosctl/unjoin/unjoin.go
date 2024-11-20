package unjoin

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

var unjoinExample = templates.Examples(i18n.T(`
		# Unjoin cluster from Kosmos control plane, e.g: 
		kosmosctl unjoin cluster --name cluster-name
		
		# Unjoin cluster from Kosmos control plane, if you need to specify a special cluster kubeconfig, e.g:
		kosmosctl unjoin cluster --name cluster-name --kubeconfig ~/kubeconfig/cluster-kubeconfig`))

type CommandUnJoinOptions struct {
	Name           string
	Namespace      string
	KubeConfig     string
	Context        string
	HostKubeConfig string
	HostContext    string

	KosmosClient        versioned.Interface
	K8sClient           kubernetes.Interface
	K8sExtensionsClient extensionsclient.Interface
}

// NewCmdUnJoin Delete resource in Kosmos control plane.
func NewCmdUnJoin(f ctlutil.Factory) *cobra.Command {
	o := &CommandUnJoinOptions{}

	cmd := &cobra.Command{
		Use:                   "unjoin",
		Short:                 i18n.T("Unjoin resource from Kosmos control plane"),
		Long:                  "",
		Example:               unjoinExample,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctlutil.CheckErr(o.Complete(f))
			ctlutil.CheckErr(o.Validate(args))
			ctlutil.CheckErr(o.Run(args))
			return nil
		},
	}

	cmd.Flags().StringVar(&o.Name, "name", "", "Specify the name of the resource to unjoin.")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", utils.DefaultNamespace, "Kosmos namespace.")
	cmd.Flags().StringVar(&o.KubeConfig, "kubeconfig", "", "Absolute path to the cluster kubeconfig file.")
	cmd.Flags().StringVar(&o.Context, "context", "", "The name of the kubeconfig context.")
	cmd.Flags().StringVar(&o.HostKubeConfig, "host-kubeconfig", "", "Absolute path to the special host kubeconfig file.")
	cmd.Flags().StringVar(&o.HostContext, "host-context", "", "The name of the host-kubeconfig context.")

	return cmd
}
func (o *CommandUnJoinOptions) Complete(_ ctlutil.Factory) error {
	hostConfig, err := utils.RestConfig(o.HostKubeConfig, o.HostContext)
	if err != nil {
		return fmt.Errorf("kosmosctl unjoin complete error, generate host config failed: %s", err)
	}

	var clusterConfig *restclient.Config

	o.KosmosClient, err = versioned.NewForConfig(hostConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl unjoin complete error, generate Kosmos client failed: %v", err)
	}

	if o.KubeConfig != "" {
		clusterConfig, err = utils.RestConfig(o.KubeConfig, o.Context)
		if err != nil {
			return fmt.Errorf("kosmosctl unjoin complete error, generate config failed: %s", err)
		}
	} else {
		var cluster *v1alpha1.Cluster
		cluster, err = o.KosmosClient.KosmosV1alpha1().Clusters().Get(context.TODO(), o.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("kosmosctl unjoin complete error, get cluster failed: %s", err)
		}
		clusterConfig, err = clientcmd.RESTConfigFromKubeConfig(cluster.Spec.Kubeconfig)
		if err != nil {
			return fmt.Errorf("kosmosctl unjoin complete error, generate clusterConfig failed: %s", err)
		}
	}

	o.K8sClient, err = kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl unjoin complete error, generate K8s basic client failed: %v", err)
	}

	o.K8sExtensionsClient, err = extensionsclient.NewForConfig(clusterConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl unjoin complete error, generate K8s extensions client failed: %v", err)
	}

	return nil
}

func (o *CommandUnJoinOptions) Validate(_ []string) error {
	if len(o.Name) == 0 {
		return fmt.Errorf("kosmosctl unjoin validate error, name is not valid")
	}

	return nil
}

func (o *CommandUnJoinOptions) Run(args []string) error {
	switch args[0] {
	case "cluster":
		err := o.runCluster()
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *CommandUnJoinOptions) runCluster() error {
	klog.Info("Start removing cluster from kosmos control plane...")
	unjoinCluster, err := o.KosmosClient.KosmosV1alpha1().Clusters().Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl unjoin run error, get cluster failed: %s", err)
		}
	}

	// delete cluster
	for {
		err := o.KosmosClient.KosmosV1alpha1().Clusters().Delete(context.TODO(), o.Name, metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				break
			}
			return fmt.Errorf("kosmosctl unjoin run error, delete cluster failed: %s", err)
		}
		time.Sleep(3 * time.Second)
	}
	klog.Info("Cluster: " + o.Name + " has been deleted.")

	if unjoinCluster.Spec.ClusterTreeOptions.Enable {
		// delete clustertree related crds
		crds := apiextensionsv1.CustomResourceDefinitionList{}
		serviceExport, err := util.GenerateCustomResourceDefinition(manifest.ServiceExport, nil)
		if err != nil {
			return err
		}
		serviceImport, err := util.GenerateCustomResourceDefinition(manifest.ServiceImport, nil)
		if err != nil {
			return err
		}
		clusterPodConvert, err := util.GenerateCustomResourceDefinition(manifest.ClusterPodConvert, nil)
		if err != nil {
			return err
		}
		podConvert, err := util.GenerateCustomResourceDefinition(manifest.PodConvert, nil)
		if err != nil {
			return err
		}
		crds.Items = append(crds.Items, *serviceImport, *serviceExport, *clusterPodConvert, *podConvert)
		for i := range crds.Items {
			err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), crds.Items[i].Name, metav1.DeleteOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.Warningf("CRD %v not found, deletion process will be skipped", crds.Items[i].Name)
					continue
				}
				return fmt.Errorf("kosmosctl unjoin run error, crd options failed: %v", err)
			}
			klog.Info("CRD " + crds.Items[i].Name + " has been deleted.")
		}
	}

	// delete rbac
	err = o.K8sClient.CoreV1().Secrets(o.Namespace).Delete(context.TODO(), utils.ControlPanelSecretName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl unjoin run error, delete secret failed: %s", err)
	}
	klog.Info("Secret: " + utils.ControlPanelSecretName + " has been deleted.")

	kosmosCR, err := util.GenerateClusterRole(manifest.KosmosClusterRole, nil)
	if err != nil {
		return fmt.Errorf("kosmosctl unjoin run error, generate clusterrole failed: %s", err)
	}
	err = o.K8sClient.RbacV1().ClusterRoles().Delete(context.TODO(), kosmosCR.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl unjoin run error, delete clusterrole failed: %s", err)
	}
	klog.Info("ClusterRole " + kosmosCR.Name + " has been deleted.")

	kosmosCRB, err := util.GenerateClusterRoleBinding(manifest.KosmosClusterRoleBinding, manifest.ClusterRoleBindingReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, generate clusterrolebinding failed: %s", err)
	}
	err = o.K8sClient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), kosmosCRB.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl unjoin run error, delete clusterrolebinding failed: %s", err)
	}
	klog.Info("ClusterRoleBinding " + kosmosCRB.Name + " has been deleted.")

	kosmosControlSA, err := util.GenerateServiceAccount(manifest.KosmosControlServiceAccount, manifest.ServiceAccountReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl unjoin run error, generate serviceaccount failed: %s", err)
	}
	err = o.K8sClient.CoreV1().ServiceAccounts(kosmosControlSA.Namespace).Delete(context.TODO(), kosmosControlSA.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kosmosctl unjoin run error, delete serviceaccount failed: %s", err)
	}
	klog.Info("ServiceAccount " + kosmosControlSA.Name + " has been deleted.")

	if unjoinCluster.Spec.ClusterLinkOptions.Enable {
		err = o.K8sClient.RbacV1().ClusterRoles().Delete(context.TODO(), utils.ClusterLink, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl unjoin run error, delete clusterrole failed: %s", err)
		}
		klog.Info("ClusterRole: " + utils.ClusterLink + " has been deleted.")

		err = o.K8sClient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), utils.ClusterLink, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl unjoin run error, delete clusterrolebinding failed: %s", err)
		}
		klog.Info("ClusterRoleBinding: " + utils.ClusterLink + " has been deleted.")

		err = o.K8sClient.CoreV1().ServiceAccounts(o.Namespace).Delete(context.TODO(), utils.OperatorName, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl unjoin run error, delete serviceaccout failed: %s", err)
		}
		klog.Info("ServiceAccount: " + utils.OperatorName + " has been deleted.")
	}

	// if cluster is not the master, delete namespace
	if o.Name != utils.DefaultClusterName {
		err = o.K8sClient.CoreV1().Namespaces().Delete(context.TODO(), o.Namespace, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("kosmosctl unjoin run error, delete namespace failed: %s", err)
		}
	}

	klog.Info("Cluster [" + o.Name + "] is removed.")
	return nil
}
