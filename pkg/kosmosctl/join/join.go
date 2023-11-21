package join

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
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
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

var joinExample = templates.Examples(i18n.T(`
        # Join cluster resource, e.g: 
        kosmosctl join cluster --name cluster-name --kubeconfig ~/kubeconfig/cluster-kubeconfig

        # Join cluster resource and turn on Clusterlink, e.g: 
        kosmosctl join cluster --name cluster-name --kubeconfig ~/kubeconfig/cluster-kubeconfig --enable-link

        # Join cluster resource and turn on Clustertree, e.g: 
        kosmosctl join cluster --name cluster-name --kubeconfig ~/kubeconfig/cluster-kubeconfig --enable-tree

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
	EnableAll            bool

	EnableLink     bool
	CNI            string
	DefaultNICName string
	NetworkType    string
	IpFamily       string
	UseProxy       string

	EnableTree bool

	KosmosClient versioned.Interface
	K8sClient    kubernetes.Interface
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
	flags.StringVarP(&o.Namespace, "namespace", "n", utils.DefaultNamespace, "Kosmos namespace.")
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Absolute path to the cluster kubeconfig file.")
	flags.StringVar(&o.HostKubeConfig, "host-kubeconfig", "", "Absolute path to the special host kubeconfig file.")
	flags.StringVar(&o.ImageRegistry, "private-image-registry", utils.DefaultImageRepository, "Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.")
	flags.BoolVar(&o.RootFlag, "root-flag", false, "Tag control cluster.")
	flags.BoolVar(&o.EnableAll, "enable-all", false, "Turn on all module.")
	flags.BoolVar(&o.EnableLink, "enable-link", false, "Turn on clusterlink.")
	flags.StringVar(&o.CNI, "cni", "", "The cluster is configured using cni and currently supports calico and flannel.")
	flags.StringVar(&o.DefaultNICName, "default-nic", "", "Set default network interface card.")
	flags.StringVar(&o.NetworkType, "network-type", utils.NetworkTypeGateway, "Set the cluster network connection mode, which supports gateway and p2p modes, gateway is used by default.")
	flags.StringVar(&o.IpFamily, "ip-family", utils.DefaultIPv4, "Specify the IP protocol version used by network devices, common IP families include IPv4 and IPv6.")
	flags.StringVar(&o.UseProxy, "use-proxy", "false", "Set whether to enable proxy.")
	flags.BoolVar(&o.EnableTree, "enable-tree", false, "Turn on clustertree.")
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
	} else {
		return fmt.Errorf("kosmosctl join complete error, arg ClusterKubeConfig is required")
	}

	return nil
}

func (o *CommandJoinOptions) Validate(args []string) error {
	if len(o.Name) == 0 {
		return fmt.Errorf("kosmosctl join validate error, name is not valid")
	}

	if len(o.Namespace) == 0 {
		return fmt.Errorf("kosmosctl join validate error, namespace is not valid")
	}

	switch args[0] {
	case "cluster":
		_, err := o.KosmosClient.KosmosV1alpha1().Clusters().Get(context.TODO(), o.Name, metav1.GetOptions{})
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
	if o.EnableAll {
		o.EnableLink = true
		o.EnableTree = true
	}

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
				Enable: o.EnableLink,
				BridgeCIDRs: v1alpha1.VxlanCIDRs{
					IP:  "220.0.0.0/8",
					IP6: "9480::0/16",
				},
				LocalCIDRs: v1alpha1.VxlanCIDRs{
					IP:  "210.0.0.0/8",
					IP6: "9470::0/16",
				},
				NetworkType: v1alpha1.NetWorkTypeGateWay,
				IPFamily:    v1alpha1.IPFamilyTypeIPV4,
			},
			ClusterTreeOptions: v1alpha1.ClusterTreeOptions{
				Enable: o.EnableTree,
			},
		},
	}

	if o.EnableLink {
		switch o.NetworkType {
		case utils.NetworkTypeP2P:
			cluster.Spec.ClusterLinkOptions.NetworkType = v1alpha1.NetworkTypeP2P
		default:
			cluster.Spec.ClusterLinkOptions.NetworkType = v1alpha1.NetWorkTypeGateWay
		}

		switch o.IpFamily {
		case utils.DefaultIPv4:
			cluster.Spec.ClusterLinkOptions.IPFamily = v1alpha1.IPFamilyTypeIPV4
		case utils.DefaultIPv6:
			cluster.Spec.ClusterLinkOptions.IPFamily = v1alpha1.IPFamilyTypeIPV6
		default:
			cluster.Spec.ClusterLinkOptions.IPFamily = v1alpha1.IPFamilyTypeALL
		}

		cluster.Spec.ClusterLinkOptions.DefaultNICName = o.DefaultNICName
		cluster.Spec.ClusterLinkOptions.CNI = o.CNI
	}

	// ToDo ClusterTree currently has no init parameters, can be expanded later.
	//if o.EnableTree {
	//
	//}

	if o.RootFlag {
		cluster.Annotations = map[string]string{
			utils.RootClusterAnnotationKey: utils.RootClusterAnnotationValue,
		}
	}

	_, err := o.KosmosClient.KosmosV1alpha1().Clusters().Create(context.TODO(), &cluster, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, create cluster failed: %s", err)
	}
	klog.Info("Cluster " + o.Name + " has been created.")

	// create ns if it does not exist
	kosmosNS := &corev1.Namespace{}
	kosmosNS.Name = o.Namespace
	_, err = o.K8sClient.CoreV1().Namespaces().Create(context.TODO(), kosmosNS, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create namespace failed: %s", err)
	}

	// create rbac
	kosmosControlSA, err := util.GenerateServiceAccount(manifest.KosmosControlServiceAccount, manifest.ServiceAccountReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, generate kosmos serviceaccount failed: %s", err)
	}
	_, err = o.K8sClient.CoreV1().ServiceAccounts(kosmosControlSA.Namespace).Create(context.TODO(), kosmosControlSA, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create kosmos serviceaccount failed: %s", err)
	}
	klog.Info("ServiceAccount " + kosmosControlSA.Name + " has been created.")

	controlPanelSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.ControlPanelSecretName,
			Namespace: o.Namespace,
		},
		Data: map[string][]byte{
			"kubeconfig": o.HostKubeConfigStream,
		},
	}
	_, err = o.K8sClient.CoreV1().Secrets(controlPanelSecret.Namespace).Create(context.TODO(), controlPanelSecret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create secret failed: %s", err)
	}
	klog.Info("Secret " + controlPanelSecret.Name + " has been created.")

	kosmosCR, err := util.GenerateClusterRole(manifest.KosmosClusterRole, nil)
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, generate clusterrole failed: %s", err)
	}
	_, err = o.K8sClient.RbacV1().ClusterRoles().Create(context.TODO(), kosmosCR, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create clusterrole failed: %s", err)
	}
	klog.Info("ClusterRole " + kosmosCR.Name + " has been created.")

	kosmosCRB, err := util.GenerateClusterRoleBinding(manifest.KosmosClusterRoleBinding, manifest.ClusterRoleBindingReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, generate clusterrolebinding failed: %s", err)
	}
	_, err = o.K8sClient.RbacV1().ClusterRoleBindings().Create(context.TODO(), kosmosCRB, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create clusterrolebinding failed: %s", err)
	}
	klog.Info("ClusterRoleBinding " + kosmosCRB.Name + " has been created.")

	kosmosOperatorSA, err := util.GenerateServiceAccount(manifest.KosmosOperatorServiceAccount, manifest.ServiceAccountReplace{
		Namespace: o.Namespace,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, generate serviceaccount failed: %s", err)
	}
	_, err = o.K8sClient.CoreV1().ServiceAccounts(kosmosOperatorSA.Namespace).Create(context.TODO(), kosmosOperatorSA, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create serviceaccount failed: %s", err)
	}
	klog.Info("ServiceAccount " + kosmosOperatorSA.Name + " has been created.")

	//ToDo Wait for all services to be running

	klog.Info("Cluster [" + o.Name + "] registration successful.")

	return nil
}
