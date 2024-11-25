package join

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	extensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
	"github.com/kosmos.io/kosmos/pkg/version"
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
	Name                  string
	Namespace             string
	ImageRegistry         string
	Version               string
	KubeConfig            string
	Context               string
	KubeConfigStream      []byte
	InnerKubeconfigStream []byte
	HostKubeConfig        string
	InnerKubeConfig       string
	HostContext           string
	HostKubeConfigStream  []byte
	WaitTime              int
	RootFlag              bool
	EnableAll             bool

	EnableLink     bool
	CNI            string
	DefaultNICName string
	NetworkType    string
	IPFamily       string
	UseProxy       string

	EnableTree bool
	LeafModel  string

	KosmosClient        versioned.Interface
	K8sClient           kubernetes.Interface
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
	flags.StringVarP(&o.Namespace, "namespace", "n", utils.DefaultNamespace, "Kosmos namespace.")
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Absolute path to the cluster kubeconfig file.")
	flags.StringVar(&o.Context, "context", "", "The name of the kubeconfig context.")
	flags.StringVar(&o.HostKubeConfig, "host-kubeconfig", "", "Absolute path to the special host kubeconfig file.")
	flags.StringVar(&o.InnerKubeConfig, "inner-kubeconfig", "", "Absolute path to the leaf cluster inner kubeconfig file.")
	flags.StringVar(&o.HostContext, "host-context", "", "The name of the host-kubeconfig context.")
	flags.StringVar(&o.ImageRegistry, "private-image-registry", utils.DefaultImageRepository, "Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.")
	flags.StringVar(&o.Version, "version", "", "image version for pull images")
	flags.BoolVar(&o.RootFlag, "root-flag", false, "Tag control cluster.")
	flags.BoolVar(&o.EnableAll, "enable-all", false, "Turn on all module.")
	flags.BoolVar(&o.EnableLink, "enable-link", false, "Turn on clusterlink.")
	flags.StringVar(&o.CNI, "cni", "", "The cluster is configured using cni and currently supports calico and flannel.")
	flags.StringVar(&o.DefaultNICName, "default-nic", "", "Set default network interface card.")
	flags.StringVar(&o.NetworkType, "network-type", "", "Set the cluster network connection mode, which supports gateway and p2p modes, gateway is used by default.")
	flags.StringVar(&o.IPFamily, "ip-family", "", "Specify the IP protocol version used by network devices, common IP families include IPv4 and IPv6.")
	flags.StringVar(&o.UseProxy, "use-proxy", "false", "Set whether to enable proxy.")
	flags.BoolVar(&o.EnableTree, "enable-tree", false, "Turn on clustertree.")
	flags.StringVar(&o.LeafModel, "leaf-model", "", "Set leaf cluster model, which supports one-to-one model.")
	flags.IntVarP(&o.WaitTime, "wait-time", "", utils.DefaultWaitTime, "Wait the specified time for the Kosmos install ready.")

	return cmd
}

func (o *CommandJoinOptions) Complete(_ ctlutil.Factory) error {
	hostConfig, err := utils.RestConfig(o.HostKubeConfig, o.HostContext)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate host rest config failed: %s", err)
	}

	if o.Version == "" {
		o.Version = fmt.Sprintf("v%s", version.GetReleaseVersion().PatchRelease())
	}

	o.KosmosClient, err = versioned.NewForConfig(hostConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate Kosmos client failed: %v", err)
	}

	// init the inner kubeconfig stream
	if len(o.InnerKubeConfig) > 0 {
		innerRawConfig, err := utils.RawConfig(o.InnerKubeConfig, o.Context)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate inner raw config failed: %s", err)
		}
		o.InnerKubeconfigStream, err = clientcmd.Write(innerRawConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, wite inner restconfig to streams failed: %s", err)
		}
	}
	if len(o.KubeConfig) > 0 {
		clusterConfig, err := utils.RestConfig(o.KubeConfig, o.Context)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate rest config failed: %s", err)
		}

		rawConfig, err := utils.RawConfig(o.KubeConfig, o.Context)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate raw config failed: %s", err)
		}

		streams, err := clientcmd.Write(rawConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, wite restconfig to streams failed: %s", err)
		}

		o.KubeConfigStream = streams
		o.K8sClient, err = kubernetes.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate K8s basic client failed: %v", err)
		}

		o.K8sExtensionsClient, err = extensionsclient.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate K8s extensions client failed: %v", err)
		}
	} else {
		return fmt.Errorf("kosmosctl join complete error, arg ClusterKubeConfig is required")
	}

	// no enable-all,enable-tree,enable-link found, make 'EnableAll' with other config
	if !o.EnableAll && !o.EnableTree && !o.EnableLink {
		// due to NetworkType or IPFamily is not empty, make EnableLink true
		if o.NetworkType != "" || o.IPFamily != "" {
			klog.Warning("due to NetworkType or IPFamily is not empty, make EnableLink ture.")
			o.EnableLink = true
		}

		// due to LeafModel is not empty, make EnableTree true
		if o.LeafModel != "" {
			klog.Warning("due to LeafModel is not empty, make EnableTree true.")
			o.EnableTree = true
		}

		// without other config, make EnableAll default true
		if !o.EnableAll && !o.EnableTree && !o.EnableLink {
			klog.Warning("All of EnableAll,EnableLink,EnableTree equals false, make EnableAll default true.")
			o.EnableAll = true
		}
	}

	if o.EnableAll {
		o.EnableLink = true
		o.EnableTree = true
	}

	if o.IPFamily == "" {
		o.IPFamily = utils.DefaultIPv4
	}

	if o.NetworkType == "" {
		o.NetworkType = utils.NetworkTypeGateway
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

	// validate: at least one of [EnableAll,EnableTree,EnableLink] need true
	if !o.EnableAll && !o.EnableTree && !o.EnableLink {
		return fmt.Errorf("kosmosctl join validate error, need at least one of enable-all,enable-tree,enable-link")
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

// CreateTreeRelatedCRDs only enable tree, create related crds
func (o *CommandJoinOptions) CreateTreeRelatedCRDs() error {
	serviceExport, err := util.GenerateCustomResourceDefinition(manifest.ServiceExport, nil)
	if err != nil {
		return err
	}
	_, err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), serviceExport, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl join run error, crd options failed: %v", err)
		}
	}
	klog.Info("Create CRD " + serviceExport.Name + " successful.")

	serviceImport, err := util.GenerateCustomResourceDefinition(manifest.ServiceImport, nil)
	if err != nil {
		return err
	}
	_, err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), serviceImport, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl join run error, crd options failed: %v", err)
		}
	}
	klog.Info("Create CRD " + serviceImport.Name + " successful.")

	clusterPodConvert, err := util.GenerateCustomResourceDefinition(manifest.ClusterPodConvert, nil)
	if err != nil {
		return err
	}
	_, err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), clusterPodConvert, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl join run error, crd options failed: %v", err)
		}
	}
	klog.Info("Create CRD " + clusterPodConvert.Name + " successful.")

	podConvert, err := util.GenerateCustomResourceDefinition(manifest.PodConvert, nil)
	if err != nil {
		return err
	}
	_, err = o.K8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), podConvert, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl join run error, crd options failed: %v", err)
		}
	}
	klog.Info("Create CRD " + podConvert.Name + " successful.")

	return nil
}

func (o *CommandJoinOptions) runCluster() error {
	klog.Info("Start registering cluster to kosmos control plane...")
	// create ns if it does not exist
	kosmosNS := &corev1.Namespace{}
	kosmosNS.Name = o.Namespace
	_, err := o.K8sClient.CoreV1().Namespaces().Create(context.TODO(), kosmosNS, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create namespace failed: %s", err)
	}

	// create cluster in control panel
	cluster := v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: o.Name,
		},
		Spec: v1alpha1.ClusterSpec{
			Kubeconfig: func() []byte {
				if len(o.InnerKubeconfigStream) != 0 {
					return o.InnerKubeconfigStream
				}
				return o.KubeConfigStream
			}(),
			Namespace:       o.Namespace,
			ImageRepository: o.ImageRegistry,
			ClusterLinkOptions: &v1alpha1.ClusterLinkOptions{
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
			ClusterTreeOptions: &v1alpha1.ClusterTreeOptions{
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

		switch o.IPFamily {
		case utils.DefaultIPv4:
			cluster.Spec.ClusterLinkOptions.IPFamily = v1alpha1.IPFamilyTypeIPV4
		case utils.DefaultIPv6:
			cluster.Spec.ClusterLinkOptions.IPFamily = v1alpha1.IPFamilyTypeIPV6
		default:
			cluster.Spec.ClusterLinkOptions.IPFamily = v1alpha1.IPFamilyTypeALL
		}

		cluster.Spec.ClusterLinkOptions.DefaultNICName = o.DefaultNICName
		cluster.Spec.ClusterLinkOptions.CNI = o.CNI

		clusterlinkOperatorSA, err := util.GenerateServiceAccount(manifest.ClusterlinkOperatorServiceAccount, manifest.ServiceAccountReplace{
			Namespace: o.Namespace,
		})
		if err != nil {
			return fmt.Errorf("kosmosctl join run error, generate serviceaccount failed: %s", err)
		}
		_, err = o.K8sClient.CoreV1().ServiceAccounts(clusterlinkOperatorSA.Namespace).Create(context.TODO(), clusterlinkOperatorSA, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl join run error, create serviceaccount failed: %s", err)
		}
		klog.Info("ServiceAccount " + clusterlinkOperatorSA.Name + " has been created.")
	}

	if o.EnableTree {
		// create ClusterTree related crds
		err := o.CreateTreeRelatedCRDs()
		if err != nil {
			return err
		}

		if len(o.LeafModel) > 0 {
			switch o.LeafModel {
			case "one-to-one":
				// ToDo Perform follow-up query based on the leaf cluster label
				nodes, err := o.K8sClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
					LabelSelector: utils.KosmosNodeJoinLabel + "=" + utils.KosmosNodeJoinValue,
				})
				if err != nil {
					return fmt.Errorf("kosmosctl join run error, list cluster node failed: %v", err)
				}
				var leafModels []v1alpha1.LeafModel
				for _, n := range nodes.Items {
					leafModel := v1alpha1.LeafModel{
						LeafNodeName: n.Name,
						Taints: []corev1.Taint{
							{
								Effect: utils.KosmosNodeTaintEffect,
								Key:    utils.KosmosNodeTaintKey,
								Value:  utils.KosmosNodeValue,
							},
						},
						NodeSelector: v1alpha1.NodeSelector{
							NodeName: n.Name,
						},
					}
					leafModels = append(leafModels, leafModel)
				}
				cluster.Spec.ClusterTreeOptions.LeafModels = leafModels
			}
		}
	}

	if o.RootFlag {
		cluster.Annotations = map[string]string{
			utils.RootClusterAnnotationKey: utils.RootClusterAnnotationValue,
		}
	}

	_, err = o.KosmosClient.KosmosV1alpha1().Clusters().Create(context.TODO(), &cluster, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, create cluster failed: %s", err)
	}
	klog.Info("Cluster " + o.Name + " has been created.")

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

	// ToDo Wait for all services to be running

	klog.Info("Cluster [" + o.Name + "] registration successful.")

	return nil
}
