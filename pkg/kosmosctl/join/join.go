package join

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	extensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
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

var joinExample = templates.Examples(i18n.T(`
        # Join cluster resource, e.g: 
        kosmosctl join cluster --name cluster-name --kubeconfig ~/kubeconfig/cluster-kubeconfig

        # Join cluster resource use param values other than default, e.g: 
        kosmosctl join cluster --name cluster-name --kubeconfig ~/kubeconfig/cluster-kubeconfig --cni cni-name --default-nic nic-name
`))

type CommandJoinOptions struct {
	KubeConfig           string
	HostKubeConfig       string
	HostKubeConfigStream []byte

	Name           string
	CNI            string
	DefaultNICName string
	ImageRegistry  string
	NetworkType    string
	IpFamily       string
	UseProxy       string
	WaitTime       int

	Client           kubernetes.Interface
	DynamicClient    *dynamic.DynamicClient
	ExtensionsClient extensionsclient.Interface
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
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Absolute path to the cluster kubeconfig file.")
	flags.StringVar(&o.HostKubeConfig, "host-kubeconfig", "", "Absolute path to the special host kubeconfig file.")
	flags.StringVar(&o.Name, "name", "", "Specify the name of the resource to join.")
	flags.StringVar(&o.CNI, "cni", "", "The cluster is configured using cni and currently supports calico and flannel.")
	flags.StringVar(&o.DefaultNICName, "default-nic", "", "Set default network interface card.")
	flags.StringVar(&o.ImageRegistry, "private-image-registry", utils.DefaultImageRepository, "Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.  In addition, you still can use --kube-image-registry to specify the registry for Kubernetes's images.")
	flags.StringVar(&o.NetworkType, "network-type", utils.NetworkTypeGateway, "Set the cluster network connection mode, which supports gateway and p2p modes, gateway is used by default.")
	flags.StringVar(&o.IpFamily, "ip-family", utils.DefaultIpFamily, "Specify the IP protocol version used by network devices, common IP families include IPv4 and IPv6.")
	flags.StringVar(&o.UseProxy, "use-proxy", "false", "Set whether to enable proxy.")
	flags.IntVarP(&o.WaitTime, "wait-time", "", 120, "Wait the specified time for the Kosmos install ready.")

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

	o.DynamicClient, err = dynamic.NewForConfig(hostConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate dynamic client failed: %s", err)
	}

	if len(o.KubeConfig) > 0 {
		clusterConfig, err = clientcmd.BuildConfigFromFlags("", o.KubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate clusterConfig failed: %s", err)
		}

		o.Client, err = kubernetes.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate basic client failed: %v", err)
		}
		o.ExtensionsClient, err = extensionsclient.NewForConfig(clusterConfig)
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
		_, err := o.DynamicClient.Resource(util.ClusterGVR).Get(context.TODO(), o.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("kosmosctl join validate error, clsuter already exists: %s", err)
			}
		}
	case "knode":
		_, err := o.DynamicClient.Resource(util.KnodeGVR).Get(context.TODO(), o.Name, metav1.GetOptions{})
		if err != nil && apierrors.IsAlreadyExists(err) {
			if apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("kosmosctl join validate error, knode already exists: %s", err)
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
	case "knode":
		err := o.runKnode()
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *CommandJoinOptions) runCluster() error {
	klog.Info("Start registering cluster to kosmos control plane...")
	// 1. create cluster in master
	clusterKubeConfigByte, err := os.ReadFile(o.KubeConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, decode cluster cr failed: %s", err)
	}
	base64ClusterKubeConfig := base64.StdEncoding.EncodeToString(clusterKubeConfigByte)
	clusterByte, err := util.GenerateCustomResource(manifest.ClusterCR, manifest.ClusterReplace{
		ClusterName:     o.Name,
		CNI:             o.CNI,
		DefaultNICName:  o.DefaultNICName,
		ImageRepository: o.ImageRegistry,
		NetworkType:     o.NetworkType,
		IpFamily:        o.IpFamily,
		KubeConfig:      base64ClusterKubeConfig,
	})
	if err != nil {
		return err
	}
	decoder := k8syaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err = decoder.Decode(clusterByte, nil, obj)
	if err != nil {
		return fmt.Errorf("(cluster) kosmosctl join run error, decode cluster cr failed: %s", err)
	}
	_, err = o.DynamicClient.Resource(util.ClusterGVR).Namespace("").Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, create cluster failed: %s", err)
	}
	klog.Info("Cluster: " + o.Name + " has been created.")

	// 2. create namespace in member
	namespace := &corev1.Namespace{}
	namespace.Name = utils.DefaultNamespace
	_, err = o.Client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create namespace failed: %s", err)
	}

	// 3. create secret in member
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
	_, err = o.Client.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create secret failed: %s", err)
	}
	klog.Info("Secret: " + secret.Name + " has been created.")

	// 4. create rbac in member
	clusterRole, err := util.GenerateClusterRole(manifest.ClusterlinkClusterRole, nil)
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, generate clusterrole failed: %s", err)
	}
	_, err = o.Client.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create clusterrole failed: %s", err)
	}
	klog.Info("ClusterRole: " + clusterRole.Name + " has been created.")

	clusterRoleBinding, err := util.GenerateClusterRoleBinding(manifest.ClusterlinkClusterRoleBinding, manifest.ClusterRoleBindingReplace{
		Namespace: utils.DefaultNamespace,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, generate clusterrolebinding failed: %s", err)
	}
	_, err = o.Client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create clusterrolebinding failed: %s", err)
	}
	klog.Info("ClusterRoleBinding: " + clusterRoleBinding.Name + " has been created.")

	// 5. create operator in member
	serviceAccount, err := util.GenerateServiceAccount(manifest.ClusterlinkOperatorServiceAccount, manifest.ServiceAccountReplace{
		Namespace: utils.DefaultNamespace,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, generate serviceaccount failed: %s", err)
	}
	_, err = o.Client.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create serviceaccount failed: %s", err)
	}
	klog.Info("ServiceAccount: " + serviceAccount.Name + " has been created.")

	deployment, err := util.GenerateDeployment(manifest.ClusterlinkOperatorDeployment, manifest.ClusterlinkDeploymentReplace{
		Namespace:       utils.DefaultNamespace,
		Version:         version.GetReleaseVersion().PatchRelease(),
		ClusterName:     o.Name,
		UseProxy:        o.UseProxy,
		ImageRepository: o.ImageRegistry,
	})
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, generate deployment failed: %s", err)
	}
	_, err = o.Client.AppsV1().Deployments(deployment.Namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create deployment failed: %s", err)
	}
	label := map[string]string{"app": deployment.Labels["app"]}
	if err = util.WaitPodReady(o.Client, deployment.Namespace, util.MapToString(label), o.WaitTime); err != nil {
		return fmt.Errorf("kosmosctl join run error, create deployment failed: %s", err)
	} else {
		klog.Info("Deployment: " + deployment.Name + " has been created.")
		klog.Info("Cluster [" + o.Name + "] registration successful.")
	}

	return nil
}

func (o *CommandJoinOptions) runKnode() error {
	klog.Info("Start registering knode to kosmos control plane...")
	clusterKubeConfigByte, err := os.ReadFile(o.KubeConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, decode knode cr failed: %s", err)
	}
	base64ClusterKubeConfig := base64.StdEncoding.EncodeToString(clusterKubeConfigByte)
	knodeByte, err := util.GenerateCustomResource(manifest.KnodeCR, manifest.KnodeReplace{
		KnodeName:       o.Name,
		KnodeKubeConfig: base64ClusterKubeConfig,
	})
	if err != nil {
		return err
	}
	decoder := k8syaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err = decoder.Decode(knodeByte, nil, obj)
	if err != nil {
		return fmt.Errorf("kosmosctl join run error, decode knode cr failed: %s", err)
	}
	_, err = o.DynamicClient.Resource(util.KnodeGVR).Namespace("").Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("kosmosctl join run error, create knode failed: %s", err)
	}
	klog.Info("Knode: " + obj.GetName() + " has been created.")

	klog.Info("Attempting to create kosmos mcs CRDs...")
	serviceExport, err := util.GenerateCustomResourceDefinition(manifest.ServiceExport, nil)
	if err != nil {
		return err
	}
	_, err = o.ExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), serviceExport, metav1.CreateOptions{})
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
	_, err = o.ExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), serviceImport, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("kosmosctl join run error, crd options failed: %v", err)
		}
	}
	klog.Info("Create CRD " + serviceImport.Name + " successful.")

	klog.Info("Knode [" + obj.GetName() + "] registration successful.")

	return nil
}
