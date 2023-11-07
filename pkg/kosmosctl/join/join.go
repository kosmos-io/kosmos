package join

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	extensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
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

var joinExample = templates.Examples(i18n.T(`
        # Join cluster resource, e.g: 
        kosmosctl join cluster --name cluster-name --kubeconfig ~/kubeconfig/cluster-kubeconfig

        # Join cluster resource use param values other than default, e.g: 
        kosmosctl join cluster --name cluster-name --kubeconfig ~/kubeconfig/cluster-kubeconfig --cni cni-name --default-nic nic-name
`))

type CommandJoinOptions struct {
	Name                 string
	Namespace            string
	ImageRegistry        string
	KubeConfig           string
	KubeConfigStream     []byte
	HostKubeConfig       string
	HostKubeConfigStream []byte
	WaitTime             int
	RootFlag             bool

	EnableLink     bool
	CNI            string
	DefaultNICName string
	NetworkType    string
	IpFamily       string
	UseProxy       string

	EnableTree bool

	KosmosClient        versioned.Interface
	K8sClient           kubernetes.Interface
	K8sDynamicClient    *dynamic.DynamicClient
	K8sExtensionsClient extensionsclient.Interface
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
			ctlutil.CheckErr(o.Validate(args))
			ctlutil.CheckErr(o.Run(args))
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&o.Name, "name", "", "Specify the name of the resource to join.")
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Absolute path to the cluster kubeconfig file.")
	flags.StringVar(&o.HostKubeConfig, "host-kubeconfig", "", "Absolute path to the special host kubeconfig file.")
	flags.StringVar(&o.ImageRegistry, "private-image-registry", utils.DefaultImageRepository, "Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.  In addition, you still can use --kube-image-registry to specify the registry for Kubernetes's images.")
	flags.BoolVar(&o.EnableLink, "enable-link", true, "Turn on clusterlink.")
	flags.StringVar(&o.CNI, "cni", "", "The cluster is configured using cni and currently supports calico and flannel.")
	flags.StringVar(&o.DefaultNICName, "default-nic", "", "Set default network interface card.")
	flags.StringVar(&o.NetworkType, "network-type", utils.NetworkTypeGateway, "Set the cluster network connection mode, which supports gateway and p2p modes, gateway is used by default.")
	flags.StringVar(&o.IpFamily, "ip-family", utils.DefaultIPv4, "Specify the IP protocol version used by network devices, common IP families include IPv4 and IPv6.")
	flags.StringVar(&o.UseProxy, "use-proxy", "false", "Set whether to enable proxy.")
	flags.BoolVar(&o.EnableTree, "enable-tree", true, "Turn on clustertree.")
	flags.IntVarP(&o.WaitTime, "wait-time", "", utils.DefaultWaitTime, "Wait the specified time for the Kosmos install ready.")

	return cmd
}

func (o *CommandJoinOptions) Complete(f ctlutil.Factory) error {
	var hostConfig *rest.Config
	var clusterConfig *rest.Config
	var err error

	if len(o.HostKubeConfig) > 0 {
		hostConfig, err = clientcmd.BuildConfigFromFlags("", o.HostKubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate hostConfig failed: %s", err)
		}
		o.HostKubeConfigStream, err = os.ReadFile(o.HostKubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, read hostConfig failed: %s", err)
		}
	} else {
		hostConfig, err = f.ToRESTConfig()
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate hostConfig failed: %s", err)
		}
		o.HostKubeConfigStream, err = os.ReadFile(filepath.Join(homedir.HomeDir(), ".kube", "config"))
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, read hostConfig failed: %s", err)
		}
	}

	o.KosmosClient, err = versioned.NewForConfig(hostConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl install complete error, generate Kosmos client failed: %v", err)
	}

	o.K8sDynamicClient, err = dynamic.NewForConfig(hostConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate dynamic client failed: %s", err)
	}

	if len(o.KubeConfig) > 0 {
		o.KubeConfigStream, err = os.ReadFile(o.KubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join run error, read KubeConfigStream failed: %s", err)
		}

		clusterConfig, err = clientcmd.BuildConfigFromFlags("", o.KubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate clusterConfig failed: %s", err)
		}

		o.K8sClient, err = kubernetes.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate basic client failed: %v", err)
		}

		o.K8sExtensionsClient, err = extensionsclient.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate extensions client failed: %v", err)
		}
	} else {
		return fmt.Errorf("kosmosctl join complete error, arg ClusterKubeConfig is required")
	}

	return nil
}

func (o *CommandJoinOptions) Validate(args []string) error {
	if len(o.Name) == 0 {
		return fmt.Errorf("kosmosctl join validate error, name is not valid")
	}

	switch args[0] {
	case "cluster":
		_, err := o.K8sDynamicClient.Resource(util.ClusterGVR).Get(context.TODO(), o.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("kosmosctl join validate error, clsuter already exists: %s", err)
			}
		}
	}

	return nil
}

func (o *CommandJoinOptions) Run(args []string) error {
	switch args[0] {
	case "cluster":
		err := o.runCluster()
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *CommandJoinOptions) runCluster() error {
	klog.Info("Start registering cluster to kosmos control plane...")
	// create cluster in control panel
	cluster := v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: o.Name,
		},
		Spec: v1alpha1.ClusterSpec{
			Kubeconfig:      o.KubeConfigStream,
			Namespace:       o.Namespace,
			ImageRepository: o.ImageRegistry,
			ClusterLinkOptions: v1alpha1.ClusterLinkOptions{
				Enable:         o.EnableLink,
				NetworkType:    v1alpha1.NetWorkTypeGateWay,
				IPFamily:       v1alpha1.IPFamilyTypeIPV4,
				CNI:            o.CNI,
				DefaultNICName: o.DefaultNICName,
			},
			ClusterTreeOptions: v1alpha1.ClusterTreeOptions{
				Enable: o.EnableTree,
			},
		},
	}

	if o.EnableLink {
		switch o.NetworkType {
		case utils.NetworkTypeGateway:
			cluster.Spec.ClusterLinkOptions.NetworkType = v1alpha1.NetWorkTypeGateWay
		case utils.NetworkTypeP2P:
			cluster.Spec.ClusterLinkOptions.NetworkType = v1alpha1.NetworkTypeP2P
		}

		switch o.IpFamily {
		case utils.DefaultIPv4:
			cluster.Spec.ClusterLinkOptions.IPFamily = v1alpha1.IPFamilyTypeIPV4
		case utils.DefaultIPv6:
			cluster.Spec.ClusterLinkOptions.IPFamily = v1alpha1.IPFamilyTypeIPV6
		}
	}

	// ToDo if enable ClusterTree

	//if o.RootFlag {
	//	cluster.Annotations[utils.RootClusterAnnotationKey] = utils.RootClusterAnnotationValue
	//}

	_, err := o.KosmosClient.KosmosV1alpha1().Clusters().Create(context.TODO(), &cluster, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, create cluster failed: %s", err)
	}
	klog.Info("Cluster " + o.Name + " has been created.")

	// create ns if it does not exist
	namespace := &corev1.Namespace{}
	namespace.Name = utils.DefaultNamespace
	_, err = o.K8sClient.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create namespace failed: %s", err)
	}

	// create rbac
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.ControlPanelSecretName,
			Namespace: utils.DefaultNamespace,
		},
		Data: map[string][]byte{
			"kubeconfig": o.HostKubeConfigStream,
		},
	}
	_, err = o.K8sClient.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create secret failed: %s", err)
	}
	klog.Info("Secret " + secret.Name + " has been created.")

	clusterRole, err := util.GenerateClusterRole(manifest.KosmosClusterRole, nil)
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, generate clusterrole failed: %s", err)
	}
	_, err = o.K8sClient.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create clusterrole failed: %s", err)
	}
	klog.Info("ClusterRole " + clusterRole.Name + " has been created.")

	clusterRoleBinding, err := util.GenerateClusterRoleBinding(manifest.KosmosClusterRoleBinding, manifest.ClusterRoleBindingReplace{
		Namespace: utils.DefaultNamespace,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, generate clusterrolebinding failed: %s", err)
	}
	_, err = o.K8sClient.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create clusterrolebinding failed: %s", err)
	}
	klog.Info("ClusterRoleBinding " + clusterRoleBinding.Name + " has been created.")

	serviceAccount, err := util.GenerateServiceAccount(manifest.KosmosOperatorServiceAccount, manifest.ServiceAccountReplace{
		Namespace: utils.DefaultNamespace,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, generate serviceaccount failed: %s", err)
	}
	_, err = o.K8sClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create serviceaccount failed: %s", err)
	}
	klog.Info("ServiceAccount " + serviceAccount.Name + " has been created.")

	//ToDo Wait for all services to be running

	klog.Info("Cluster [" + o.Name + "] registration successful.")

	return nil
}
