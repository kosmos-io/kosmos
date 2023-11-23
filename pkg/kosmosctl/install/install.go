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

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/cert"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/join"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/version"
)

var installExample = templates.Examples(i18n.T(`
		# Install all module to Kosmos control plane, e.g: 
		kosmosctl install --cni cni-name --default-nic nic-name
		
		# Install Kosmos control plane, if you need to specify a special control plane cluster kubeconfig, e.g: 
		kosmosctl install --kubeconfig ~/kubeconfig/cluster-kubeconfig
		
		# Install clustertree module to Kosmos control plane, e.g: 
		kosmosctl install -m clustertree
		
		# Install clusterlink module to Kosmos control plane and set the necessary parameters, e.g: 
		kosmosctl install -m clusterlink --cni cni-name --default-nic nic-name
		
		# Install coredns module to Kosmos control plane, e.g: 
		kosmosctl install -m coredns`))

type CommandInstallOptions struct {
	Namespace            string
	ImageRegistry        string
	Version              string
	Module               string
	HostKubeConfig       string
	HostKubeConfigStream []byte
	WaitTime             int

	CNI            string
	DefaultNICName string
	NetworkType    string
	IpFamily       string
	UseProxy       string

	KosmosClient        versioned.Interface
	K8sClient           kubernetes.Interface
	K8sExtensionsClient extensionsclient.Interface

	CertEncode string
	KeyEncode  string
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
	flags.StringVarP(&o.Module, "module", "m", utils.All, "Kosmos specify the module to install.")
	flags.StringVar(&o.HostKubeConfig, "kubeconfig", "", "Absolute path to the special kubeconfig file.")
	flags.StringVar(&o.CNI, "cni", "", "The cluster is configured using cni and currently supports calico and flannel.")
	flags.StringVar(&o.DefaultNICName, "default-nic", "", "Set default network interface card.")
	flags.StringVar(&o.NetworkType, "network-type", utils.NetworkTypeGateway, "Set the cluster network connection mode, which supports gateway and p2p modes, gateway is used by default.")
	flags.StringVar(&o.IpFamily, "ip-family", string(v1alpha1.IPFamilyTypeIPV4), "Specify the IP protocol version used by network devices, common IP families include IPv4 and IPv6.")
	flags.StringVar(&o.UseProxy, "use-proxy", "false", "Set whether to enable proxy.")
	flags.IntVarP(&o.WaitTime, "wait-time", "", utils.DefaultWaitTime, "Wait the specified time for the Kosmos install ready.")

	flags.StringVar(&o.CertEncode, "cert-encode", cert.GetCrtEncode(), "cert base64 string for node server.")
	flags.StringVar(&o.KeyEncode, "key-encode", cert.GetKeyEncode(), "key base64 string for node server.")

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

	o.KosmosClient, err = versioned.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl install complete error, generate Kosmos client failed: %v", err)
	}

	o.K8sClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl install complete error, generate K8s basic client failed: %v", err)
	}

	o.K8sExtensionsClient, err = extensionsclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("kosmosctl install complete error, generate K8s extensions client failed: %v", err)
	}

	return nil
}

func (o *CommandInstallOptions) Validate() error {
	if len(o.Namespace) == 0 {
		return fmt.Errorf("kosmosctl install validate error, namespace is not valid")
	}

	return nil
}

func (o *CommandInstallOptions) Run() error {
	klog.Info("Kosmos starts installing.")
	switch o.Module {
	case utils.CoreDNS:
		err := o.runCoreDNS()
		if err != nil {
			return err
		}
		util.CheckInstall("CoreDNS")
	case utils.ClusterLink:
		err := o.runClusterlink()
		if err != nil {
			return err
		}
		err = o.createControlCluster()
		if err != nil {
			return err
		}
		util.CheckInstall("Clusterlink")
	case utils.ClusterTree:
		err := o.runClustertree()
		if err != nil {
			return err
		}
		err = o.createControlCluster()
		if err != nil {
			return err
		}
		util.CheckInstall("Clustertree")
	case utils.All:
		err := o.runClusterlink()
		if err != nil {
			return err
		}
		err = o.runClustertree()
		if err != nil {
			return err
		}
		err = o.createControlCluster()
		if err != nil {
			return err
		}
		util.CheckInstall("Clusterlink && Clustertree")
	}

	return nil
}

func (o *CommandInstallOptions) runClusterlink() error {
	klog.Info("Start creating Kosmos-Clusterlink...")
	namespace := &corev1.Namespace{}
	namespace.Name = o.Namespace
	_, err := o.K8sClient.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, namespace options failed: %v", err)
		}
	}
	klog.Info("Namespace " + namespace.Name + " has been created.")

	klog.Info("Start creating Kosmos-Clusterlink network-manager RBAC...")
	networkManagerSA, err := util.GenerateServiceAccount(manifest.ClusterlinkNetworkManagerServiceAccount, manifest.ServiceAccountReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.K8sClient.CoreV1().ServiceAccounts(o.Namespace).Create(context.TODO(), networkManagerSA, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, network-manager serviceaccount options failed: %v", err)
		}
	}
	klog.Info("ServiceAccount " + networkManagerSA.Name + " has been created.")

	networkManagerCR, err := util.GenerateClusterRole(manifest.ClusterlinkNetworkManagerClusterRole, nil)
	if err != nil {
		return err
	}
	_, err = o.K8sClient.RbacV1().ClusterRoles().Create(context.TODO(), networkManagerCR, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, network-manager clusterrole options failed: %v", err)
		}
	}
	klog.Info("ClusterRole " + networkManagerCR.Name + " has been created.")

	networkManagerCRB, err := util.GenerateClusterRoleBinding(manifest.ClusterlinkNetworkManagerClusterRoleBinding, manifest.ClusterRoleBindingReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.K8sClient.RbacV1().ClusterRoleBindings().Create(context.TODO(), networkManagerCRB, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, network-manager clusterrolebinding options failed: %v", err)
		}
	}
	klog.Info("ClusterRoleBinding " + networkManagerCRB.Name + " has been created.")

	klog.Info("Attempting to create Kosmos-Clusterlink CRDs...")
	crds := apiextensionsv1.CustomResourceDefinitionList{}
	clusterlinkCluster, err := util.GenerateCustomResourceDefinition(manifest.Cluster, manifest.CRDReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	clusterlinkClusterNode, err := util.GenerateCustomResourceDefinition(manifest.ClusterNode, nil)
	if err != nil {
		return err
	}
	clusterlinkNodeConfig, err := util.GenerateCustomResourceDefinition(manifest.NodeConfig, nil)
	if err != nil {
		return err
	}
	crds.Items = append(crds.Items, *clusterlinkCluster, *clusterlinkClusterNode, *clusterlinkNodeConfig)
	for i := range crds.Items {
		_, err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), &crds.Items[i], metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				klog.Warningf("CRD %v is existed, creation process will skip", &crds.Items[i].Name)
				continue
			} else {
				return fmt.Errorf("kosmosctl install clusterlink run error, crd options failed: %v", err)
			}
		}
		klog.Info("Create CRD " + crds.Items[i].Name + " successful.")
	}

	klog.Info("Start creating Kosmos-Clusterlink network-manager Deployment...")
	networkManagerDeploy, err := util.GenerateDeployment(manifest.ClusterlinkNetworkManagerDeployment, manifest.DeploymentReplace{
		Namespace:       o.Namespace,
		ImageRepository: o.ImageRegistry,
		Version:         version.GetReleaseVersion().PatchRelease(),
	})
	if err != nil {
		return err
	}
	_, err = o.K8sClient.AppsV1().Deployments(o.Namespace).Create(context.Background(), networkManagerDeploy, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clusterlink run error, network-manager deployment options failed: %v", err)
		}
	}
	networkManagerLabel := map[string]string{"app": networkManagerDeploy.Labels["app"]}
	if err = util.WaitPodReady(o.K8sClient, networkManagerDeploy.Namespace, util.MapToString(networkManagerLabel), o.WaitTime); err != nil {
		return fmt.Errorf("kosmosctl install clusterlink run error, network-manager deployment options failed: %v", err)
	} else {
		klog.Info("Deployment " + networkManagerDeploy.Name + " has been created.")
	}

	operatorDeploy, err := util.GenerateDeployment(manifest.KosmosOperatorDeployment, manifest.DeploymentReplace{
		Namespace:       o.Namespace,
		Version:         version.GetReleaseVersion().PatchRelease(),
		UseProxy:        o.UseProxy,
		ImageRepository: o.ImageRegistry,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl install operator run error, generate deployment failed: %s", err)
	}
	_, err = o.K8sClient.AppsV1().Deployments(operatorDeploy.Namespace).Get(context.TODO(), operatorDeploy.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = o.createOperator()
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("kosmosctl install operator run error, get operator deployment failed: %s", err)
		}
	}

	return nil
}

func (o *CommandInstallOptions) runClustertree() error {
	klog.Info("Start creating kosmos-clustertree...")
	namespace := &corev1.Namespace{}
	namespace.Name = o.Namespace
	_, err := o.K8sClient.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, namespace options failed: %v", err)
		}
	}
	klog.Info("Namespace " + o.Namespace + " has been created.")

	klog.Info("Start creating kosmos-clustertree ServiceAccount...")
	clustertreeSA, err := util.GenerateServiceAccount(manifest.ClusterTreeServiceAccount, manifest.ServiceAccountReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.K8sClient.CoreV1().ServiceAccounts(o.Namespace).Create(context.TODO(), clustertreeSA, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, serviceaccount options failed: %v", err)
		}
	}
	klog.Info("ServiceAccount " + clustertreeSA.Name + " has been created.")

	klog.Info("Start creating kosmos-clustertree ClusterRole...")
	clustertreeCR, err := util.GenerateClusterRole(manifest.ClusterTreeClusterRole, nil)
	if err != nil {
		return err
	}
	_, err = o.K8sClient.RbacV1().ClusterRoles().Create(context.TODO(), clustertreeCR, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, clusterrole options failed: %v", err)
		}
	}
	klog.Info("ClusterRole " + clustertreeCR.Name + " has been created.")

	klog.Info("Start creating kosmos-clustertree ClusterRoleBinding...")
	clustertreeCRB, err := util.GenerateClusterRoleBinding(manifest.ClusterTreeClusterRoleBinding, manifest.ClusterRoleBindingReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.K8sClient.RbacV1().ClusterRoleBindings().Create(context.TODO(), clustertreeCRB, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, clusterrolebinding options failed: %v", err)
		}
	}
	klog.Info("ClusterRoleBinding " + clustertreeCRB.Name + " has been created.")

	klog.Info("Attempting to create kosmos-clustertree CRDs...")
	clustertreeCluster, err := util.GenerateCustomResourceDefinition(manifest.Cluster, nil)
	if err != nil {
		return err
	}
	_, err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), clustertreeCluster, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Warningf("CRD %v is existed, creation process will skip", clustertreeCluster.Name)
		} else {
			return fmt.Errorf("kosmosctl install clustertree run error, crd options failed: %v", err)
		}
	}
	klog.Info("Create CRD " + clustertreeCluster.Name + " successful.")

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
	_, err = o.K8sClient.CoreV1().ConfigMaps(o.Namespace).Create(context.TODO(), clustertreeConfigMap, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, configmap options failed: %v", err)
		}
	}
	klog.Info("ConfigMap host-kubeconfig has been created.")

	klog.Info("Start creating kosmos-clustertree secret")
	clustertreeSecret, err := util.GenerateSecret(manifest.ClusterTreeClusterManagerSecret, manifest.SecretReplace{
		Namespace: o.Namespace,
		Cert:      o.CertEncode,
		Key:       o.KeyEncode,
	})
	if err != nil {
		return err
	}
	_, err = o.K8sClient.CoreV1().Secrets(o.Namespace).Create(context.Background(), clustertreeSecret, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, secret options failed: %v", err)
		}
	}
	klog.Info("Secret has been created. ")

	klog.Info("Start creating kosmos-clustertree Deployment...")
	clustertreeDeploy, err := util.GenerateDeployment(manifest.ClusterTreeClusterManagerDeployment, manifest.DeploymentReplace{
		Namespace:       o.Namespace,
		ImageRepository: o.ImageRegistry,
		Version:         version.GetReleaseVersion().PatchRelease(),
	})
	if err != nil {
		return err
	}
	_, err = o.K8sClient.AppsV1().Deployments(o.Namespace).Create(context.Background(), clustertreeDeploy, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install clustertree run error, deployment options failed: %v", err)
		}
	}
	label := map[string]string{"app": clustertreeDeploy.Labels["app"]}
	if err = util.WaitPodReady(o.K8sClient, clustertreeDeploy.Namespace, util.MapToString(label), o.WaitTime); err != nil {
		return fmt.Errorf("kosmosctl install clustertree run error, deployment options failed: %v", err)
	} else {
		klog.Info("Deployment clustertree-cluster-manager has been created.")
	}

	operatorDeploy, err := util.GenerateDeployment(manifest.KosmosOperatorDeployment, manifest.DeploymentReplace{
		Namespace:       o.Namespace,
		Version:         version.GetReleaseVersion().PatchRelease(),
		UseProxy:        o.UseProxy,
		ImageRepository: o.ImageRegistry,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl install operator run error, operator generate deployment failed: %s", err)
	}
	_, err = o.K8sClient.AppsV1().Deployments(operatorDeploy.Namespace).Get(context.TODO(), operatorDeploy.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = o.createOperator()
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("kosmosctl install operator run error, operator get deployment failed: %s", err)
		}
	}

	return nil
}

func (o *CommandInstallOptions) createOperator() error {
	klog.Info("Start creating Kosmos-Operator...")
	operatorDeploy, err := util.GenerateDeployment(manifest.KosmosOperatorDeployment, manifest.DeploymentReplace{
		Namespace:       o.Namespace,
		Version:         version.GetReleaseVersion().PatchRelease(),
		UseProxy:        o.UseProxy,
		ImageRepository: o.ImageRegistry,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl install operator run error, operator generate deployment failed: %s", err)
	}
	_, err = o.K8sClient.AppsV1().Deployments(operatorDeploy.Namespace).Create(context.TODO(), operatorDeploy, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl install operator run error, operator options deployment failed: %s", err)
	}

	operatorSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.ControlPanelSecretName,
			Namespace: o.Namespace,
		},
		Data: map[string][]byte{
			"kubeconfig": o.HostKubeConfigStream,
		},
	}
	_, err = o.K8sClient.CoreV1().Secrets(operatorSecret.Namespace).Create(context.TODO(), operatorSecret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl install operator run error, operator options secret failed: %s", err)
	}

	operatorCR, err := util.GenerateClusterRole(manifest.KosmosClusterRole, nil)
	if err != nil {
		return fmt.Errorf("kosmosctl install operator run error, generate operator clusterrole failed: %s", err)
	}
	_, err = o.K8sClient.RbacV1().ClusterRoles().Create(context.TODO(), operatorCR, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl install operator run error, operator options clusterrole failed: %s", err)
	}

	operatorCRB, err := util.GenerateClusterRoleBinding(manifest.KosmosClusterRoleBinding, manifest.ClusterRoleBindingReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl install operator run error, generate operator clusterrolebinding failed: %s", err)
	}
	_, err = o.K8sClient.RbacV1().ClusterRoleBindings().Create(context.TODO(), operatorCRB, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl install operator run error, operator options clusterrolebinding failed: %s", err)
	}

	operatorSA, err := util.GenerateServiceAccount(manifest.KosmosOperatorServiceAccount, manifest.ServiceAccountReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl install operator run error, generate operator serviceaccount failed: %s", err)
	}
	_, err = o.K8sClient.CoreV1().ServiceAccounts(operatorSA.Namespace).Create(context.TODO(), operatorSA, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl install clusterlink run error, operator options serviceaccount failed: %s", err)
	}

	operatorLabel := map[string]string{"app": operatorDeploy.Labels["app"]}
	if err = util.WaitPodReady(o.K8sClient, operatorDeploy.Namespace, util.MapToString(operatorLabel), o.WaitTime); err != nil {
		return fmt.Errorf("kosmosctl install operator run error, operator options deployment failed: %s", err)
	} else {
		klog.Info("Operator " + operatorDeploy.Name + " has been created.")
	}

	return nil
}

func (o *CommandInstallOptions) createControlCluster() error {
	switch o.Module {
	case utils.ClusterLink:
		controlCluster, err := o.KosmosClient.KosmosV1alpha1().Clusters().Get(context.TODO(), utils.DefaultClusterName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				clusterArgs := []string{"cluster"}
				joinOptions := join.CommandJoinOptions{
					Name:             utils.DefaultClusterName,
					Namespace:        o.Namespace,
					ImageRegistry:    o.ImageRegistry,
					KubeConfigStream: o.HostKubeConfigStream,
					WaitTime:         o.WaitTime,
					KosmosClient:     o.KosmosClient,
					K8sClient:        o.K8sClient,
					RootFlag:         true,
					EnableLink:       true,
					CNI:              o.CNI,
					DefaultNICName:   o.DefaultNICName,
					NetworkType:      o.NetworkType,
					IpFamily:         o.IpFamily,
					UseProxy:         o.UseProxy,
				}

				err = joinOptions.Run(clusterArgs)
				if err != nil {
					return fmt.Errorf("kosmosctl install run error, join control panel cluster failed: %s", err)
				}
			} else {
				return fmt.Errorf("kosmosctl install run error, get control panel cluster failed: %s", err)
			}
		}

		if len(controlCluster.Name) > 0 {
			if !controlCluster.Spec.ClusterLinkOptions.Enable {
				controlCluster.Spec.ClusterLinkOptions.Enable = true
				controlCluster.Spec.ClusterLinkOptions.CNI = o.CNI
				controlCluster.Spec.ClusterLinkOptions.DefaultNICName = o.DefaultNICName
				switch o.NetworkType {
				case utils.NetworkTypeGateway:
					controlCluster.Spec.ClusterLinkOptions.NetworkType = v1alpha1.NetWorkTypeGateWay
				case utils.NetworkTypeP2P:
					controlCluster.Spec.ClusterLinkOptions.NetworkType = v1alpha1.NetworkTypeP2P
				}

				switch o.IpFamily {
				case utils.DefaultIPv4:
					controlCluster.Spec.ClusterLinkOptions.IPFamily = v1alpha1.IPFamilyTypeIPV4
				case utils.DefaultIPv6:
					controlCluster.Spec.ClusterLinkOptions.IPFamily = v1alpha1.IPFamilyTypeIPV6
				}
				_, err = o.KosmosClient.KosmosV1alpha1().Clusters().Update(context.TODO(), controlCluster, metav1.UpdateOptions{})
				if err != nil {
					klog.Infof("ControlCluster-Link: ", controlCluster)
					return fmt.Errorf("kosmosctl install clusterlink run error, update control panel cluster failed: %s", err)
				}
			}
		}
	case utils.ClusterTree:
		controlCluster, err := o.KosmosClient.KosmosV1alpha1().Clusters().Get(context.TODO(), utils.DefaultClusterName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				clusterArgs := []string{"cluster"}
				joinOptions := join.CommandJoinOptions{
					Name:             utils.DefaultClusterName,
					Namespace:        o.Namespace,
					ImageRegistry:    o.ImageRegistry,
					KubeConfigStream: o.HostKubeConfigStream,
					WaitTime:         o.WaitTime,
					KosmosClient:     o.KosmosClient,
					K8sClient:        o.K8sClient,
					RootFlag:         true,
					EnableTree:       true,
				}

				err = joinOptions.Run(clusterArgs)
				if err != nil {
					return fmt.Errorf("kosmosctl install run error, join control panel cluster failed: %s", err)
				}
			} else {
				return fmt.Errorf("kosmosctl install run error, get control panel cluster failed: %s", err)
			}
		}

		if len(controlCluster.Name) > 0 {
			if !controlCluster.Spec.ClusterTreeOptions.Enable {
				controlCluster.Spec.ClusterTreeOptions.Enable = true
				_, err = o.KosmosClient.KosmosV1alpha1().Clusters().Update(context.TODO(), controlCluster, metav1.UpdateOptions{})
				if err != nil {
					klog.Infof("ControlCluster-Tree: ", controlCluster)
					return fmt.Errorf("kosmosctl install clustertree run error, update control panel cluster failed: %s", err)
				}
			}
		}
	case utils.All:
		controlCluster, err := o.KosmosClient.KosmosV1alpha1().Clusters().Get(context.TODO(), utils.DefaultClusterName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				clusterArgs := []string{"cluster"}
				joinOptions := join.CommandJoinOptions{
					Name:             utils.DefaultClusterName,
					Namespace:        o.Namespace,
					ImageRegistry:    o.ImageRegistry,
					KubeConfigStream: o.HostKubeConfigStream,
					WaitTime:         o.WaitTime,
					KosmosClient:     o.KosmosClient,
					K8sClient:        o.K8sClient,
					RootFlag:         true,
					EnableLink:       true,
					CNI:              o.CNI,
					DefaultNICName:   o.DefaultNICName,
					NetworkType:      o.NetworkType,
					IpFamily:         o.IpFamily,
					UseProxy:         o.UseProxy,
					EnableTree:       true,
				}

				err = joinOptions.Run(clusterArgs)
				if err != nil {
					return fmt.Errorf("kosmosctl install run error, join control panel cluster failed: %s", err)
				}
			} else {
				return fmt.Errorf("kosmosctl install run error, get control panel cluster failed: %s", err)
			}
		}

		if len(controlCluster.Name) > 0 {
			if !controlCluster.Spec.ClusterTreeOptions.Enable || !controlCluster.Spec.ClusterLinkOptions.Enable {
				controlCluster.Spec.ClusterTreeOptions.Enable = true
				controlCluster.Spec.ClusterLinkOptions.Enable = true
				controlCluster.Spec.ClusterLinkOptions.CNI = o.CNI
				controlCluster.Spec.ClusterLinkOptions.DefaultNICName = o.DefaultNICName
				switch o.NetworkType {
				case utils.NetworkTypeGateway:
					controlCluster.Spec.ClusterLinkOptions.NetworkType = v1alpha1.NetWorkTypeGateWay
				case utils.NetworkTypeP2P:
					controlCluster.Spec.ClusterLinkOptions.NetworkType = v1alpha1.NetworkTypeP2P
				}

				switch o.IpFamily {
				case utils.DefaultIPv4:
					controlCluster.Spec.ClusterLinkOptions.IPFamily = v1alpha1.IPFamilyTypeIPV4
				case utils.DefaultIPv6:
					controlCluster.Spec.ClusterLinkOptions.IPFamily = v1alpha1.IPFamilyTypeIPV6
				}
				_, err = o.KosmosClient.KosmosV1alpha1().Clusters().Update(context.TODO(), controlCluster, metav1.UpdateOptions{})
				if err != nil {
					klog.Infof("ControlCluster-All: ", controlCluster)
					return fmt.Errorf("kosmosctl install clustertree run error, update control panel cluster failed: %s", err)
				}
			}
		}
	}

	return nil
}

func (o *CommandInstallOptions) runCoreDNS() error {
	klog.Info("Start creating kosmos-coredns...")
	namespace := &corev1.Namespace{}
	namespace.Name = o.Namespace
	_, err := o.K8sClient.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
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
	_, err = o.K8sClient.CoreV1().ServiceAccounts(o.Namespace).Create(context.TODO(), sa, metav1.CreateOptions{})
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
	_, err = o.K8sClient.RbacV1().ClusterRoles().Create(context.TODO(), cRole, metav1.CreateOptions{})
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
	_, err = o.K8sClient.RbacV1().ClusterRoleBindings().Create(context.TODO(), crb, metav1.CreateOptions{})
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
	_, err = o.K8sClient.CoreV1().ConfigMaps(o.Namespace).Create(context.TODO(), coreFile, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns coreFile run error, configmap options failed: %v", err)
		}
	}
	klog.Infof("ConfigMap %s has been created.", coreFile.Name)

	customerHosts, err := util.GenerateConfigMap(manifest.CorednsCustomerHosts, manifest.ConfigmapReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.K8sClient.CoreV1().ConfigMaps(o.Namespace).Create(context.TODO(), customerHosts, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns customerHosts run error, configmap options failed: %v", err)
		}
	}
	klog.Infof("ConfigMap %s has been created.", customerHosts.Name)

	klog.Info("Attempting to create coredns CRDs, coredns reuses clusterlink's cluster CRD")
	crd, err := util.GenerateCustomResourceDefinition(manifest.Cluster, manifest.CRDReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return err
	}
	_, err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), crd, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Warningf("CRD %v is existed, creation process will skip", crd.Name)
		} else {
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
	_, err = o.K8sClient.AppsV1().Deployments(o.Namespace).Create(context.Background(), deploy, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns run error, deployment options failed: %v", err)
		}
	}
	if err = util.WaitDeploymentReady(o.K8sClient, deploy, o.WaitTime); err != nil {
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
	_, err = o.K8sClient.CoreV1().Services(o.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl install coredns run error, service options failed: %v", err)
		}
	}
	klog.Infof("Create service %s successful.", svc.Name)

	return nil
}
