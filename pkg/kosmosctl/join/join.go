package join

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
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
        # Join cluster resource from a directory containing cluster.yaml, e.g: 
        kosmosctl join cluster --cluster-name=[cluster-name] --master-kubeconfig=[master-kubeconfig] --cluster-kubeconfig=[cluster-kubeconfig]
        
        # Join cluster resource without master-kubeconfig, e.g: 
        kosmosctl join cluster --cluster-name=[cluster-name] --cluster-kubeconfig=[cluster-kubeconfig]

        # Join knode resource, e.g: 
        kosmosctl join knode --knode-name=[knode-name] --master-kubeconfig=[master-kubeconfig] --cluster-kubeconfig=[cluster-kubeconfig]

        # Join knode resource without master-kubeconfig, e.g: 
        kosmosctl join knode --knode-name=[knode-name] --cluster-kubeconfig=[cluster-kubeconfig]
`))

type CommandJoinOptions struct {
	MasterKubeConfig       string
	MasterKubeConfigStream []byte
	ClusterKubeConfig      string

	ClusterName    string
	CNI            string
	DefaultNICName string
	ImageRegistry  string
	NetworkType    string
	UseProxy       string
	WaitTime       int

	KnodeName string

	Client        kubernetes.Interface
	DynamicClient *dynamic.DynamicClient
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
	flags.StringVar(&o.MasterKubeConfig, "master-kubeconfig", "", "Absolute path to the master kubeconfig file.")
	flags.StringVar(&o.ClusterKubeConfig, "cluster-kubeconfig", "", "Absolute path to the cluster kubeconfig file.")
	flags.StringVar(&o.ClusterName, "cluster-name", "", "Specify the name of the member cluster to join.")
	flags.StringVar(&o.CNI, "cni", "", "The cluster is configured using cni and currently supports calico and flannel.")
	flags.StringVar(&o.DefaultNICName, "default-nic", "", "Set default network interface card.")
	flags.StringVar(&o.ImageRegistry, "private-image-registry", utils.DefaultImageRepository, "Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.  In addition, you still can use --kube-image-registry to specify the registry for Kubernetes's images.")
	flags.StringVar(&o.NetworkType, "network-type", "gateway", "Set the cluster network connection mode, which supports gateway and p2p modes. Gateway is used by default.")
	flags.StringVar(&o.KnodeName, "knode-name", "", "Specify the name of the knode to join.")
	flags.StringVar(&o.UseProxy, "use-proxy", "false", "Set whether to enable proxy.")
	flags.IntVarP(&o.WaitTime, "wait-time", "", 120, "Wait the specified time for the Kosmos install ready.")

	return cmd
}

func (o *CommandJoinOptions) Complete(f ctlutil.Factory) error {
	var masterConfig *rest.Config
	var clusterConfig *rest.Config
	var err error

	if len(o.MasterKubeConfig) > 0 {
		masterConfig, err = clientcmd.BuildConfigFromFlags("", o.MasterKubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate masterConfig failed: %s", err)
		}
		o.MasterKubeConfigStream, err = os.ReadFile(o.MasterKubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, read masterconfig failed: %s", err)
		}
	} else {
		masterConfig, err = f.ToRESTConfig()
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate masterConfig failed: %s", err)
		}
		o.MasterKubeConfigStream, err = os.ReadFile(filepath.Join(homedir.HomeDir(), ".kube", "config"))
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, read masterconfig failed: %s", err)
		}
	}

	o.DynamicClient, err = dynamic.NewForConfig(masterConfig)
	if err != nil {
		return fmt.Errorf("kosmosctl join complete error, generate dynamic client failed: %s", err)
	}

	if len(o.ClusterKubeConfig) > 0 {
		clusterConfig, err = clientcmd.BuildConfigFromFlags("", o.ClusterKubeConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate clusterConfig failed: %s", err)
		}

		o.Client, err = kubernetes.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("kosmosctl join complete error, generate basic client failed: %v", err)
		}
	}

	return nil
}

func (o *CommandJoinOptions) Validate(args []string) error {
	switch args[0] {
	case "cluster":
		_, err := o.DynamicClient.Resource(util.ClusterGVR).Get(context.TODO(), o.ClusterName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("kosmosctl join validate error, clsuter already exists: %s", err)
			}
		}
	case "knode":
		_, err := o.DynamicClient.Resource(util.KnodeGVR).Get(context.TODO(), o.KnodeName, metav1.GetOptions{})
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
	clusterByte, err := util.GenerateCustomResource(manifest.ClusterCR, manifest.ClusterReplace{
		ClusterName:     o.ClusterName,
		CNI:             o.CNI,
		DefaultNICName:  o.DefaultNICName,
		ImageRepository: o.ImageRegistry,
		NetworkType:     o.NetworkType,
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
		return fmt.Errorf("(cluster) kosmosctl join run error, create cluster failed: %s", err)
	}

	// 2. create namespace in member
	namespace := &corev1.Namespace{}
	namespace.Name = utils.DefaultNamespace
	_, err = o.Client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster namespace) kosmosctl join run error, create namespace failed: %s", err)
	}

	// 3. create secret in member
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.ControlPanelSecretName,
			Namespace: utils.DefaultNamespace,
		},
		Data: map[string][]byte{
			"kubeconfig": o.MasterKubeConfigStream,
		},
	}
	_, err = o.Client.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster secret) kosmosctl join run error, create secret failed: %s", err)
	}

	// 4. create rbac in member
	clusterRole, err := util.GenerateClusterRole(manifest.ClusterlinkClusterRole, nil)
	if err != nil {
		return fmt.Errorf("(cluster rbac) kosmosctl join run error, generate clusterrole failed: %s", err)
	}
	_, err = o.Client.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster rbac) kosmosctl join run error, create clusterrole failed: %s", err)
	}
	clusterRoleBinding, err := util.GenerateClusterRoleBinding(manifest.ClusterlinkClusterRoleBinding, manifest.ClusterRoleBindingReplace{
		Namespace: utils.DefaultNamespace,
	})
	if err != nil {
		return fmt.Errorf("(cluster rbac) kosmosctl join run error, generate clusterrolebinding failed: %s", err)
	}
	_, err = o.Client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster rbac) kosmosctl join run error, create clusterrolebinding failed: %s", err)
	}

	// 5. create operator in member
	serviceAccount, err := util.GenerateServiceAccount(manifest.ClusterlinkOperatorServiceAccount, manifest.ServiceAccountReplace{
		Namespace: utils.DefaultNamespace,
	})
	if err != nil {
		return fmt.Errorf("(cluster operator) kosmosctl join run error, generate serviceaccount failed: %s", err)
	}
	_, err = o.Client.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster operator) kosmosctl join run error, create serviceaccount failed: %s", err)
	}

	deployment, err := util.GenerateDeployment(manifest.ClusterlinkOperatorDeployment, manifest.ClusterlinkDeploymentReplace{
		Namespace:       utils.DefaultNamespace,
		Version:         version.GetReleaseVersion().PatchRelease(),
		ClusterName:     o.ClusterName,
		UseProxy:        o.UseProxy,
		ImageRepository: o.ImageRegistry,
	})
	if err != nil {
		return fmt.Errorf("(cluster operator) kosmosctl join run error, generate deployment failed: %s", err)
	}
	_, err = o.Client.AppsV1().Deployments(deployment.Namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(cluster operator) kosmosctl join run error, create deployment failed: %s", err)
	}
	if err = util.WaitDeploymentReady(o.Client, deployment, o.WaitTime); err != nil {
		return fmt.Errorf("(cluster operator) kosmosctl join run error, create deployment failed: %s", err)
	} else {
		klog.Info("Cluster registration successful.")
	}

	return nil
}

func (o *CommandJoinOptions) runKnode() error {
	klog.Info("Start registering knode to kosmos control plane...")
	clusterKubeConfigByte, err := os.ReadFile(o.ClusterKubeConfig)
	if err != nil {
		return fmt.Errorf("(knode) kosmosctl join run error, decode knode cr failed: %s", err)
	}
	base64ClusterKubeConfig := base64.StdEncoding.EncodeToString(clusterKubeConfigByte)
	knodeByte, err := util.GenerateCustomResource(manifest.KnodeCR, manifest.KnodeReplace{
		KnodeName:       o.KnodeName,
		KnodeKubeConfig: base64ClusterKubeConfig,
	})
	if err != nil {
		return err
	}
	decoder := k8syaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err = decoder.Decode(knodeByte, nil, obj)
	if err != nil {
		return fmt.Errorf("(knode) kosmosctl join run error, decode knode cr failed: %s", err)
	}
	_, err = o.DynamicClient.Resource(util.KnodeGVR).Namespace("").Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("(knode) kosmosctl join run error, create knode failed: %s", err)
	}
	klog.Info("Knode registration successful.")

	return nil
}
