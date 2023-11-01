package option

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	cmdOptions "github.com/kosmos.io/kosmos/cmd/clusterlink/operator/app/options"
	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/version"
)

// AddonOption for cluster
type AddonOption struct {
	clusterlinkv1alpha1.Cluster

	Version  string
	UseProxy bool

	KubeClientSet          *kubernetes.Clientset
	ControlPanelKubeConfig *clientcmdapi.Config
}

func (o *AddonOption) buildClusterConfig(opts *cmdOptions.Options) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %s", err.Error())
	}

	clusterClientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	o.KubeClientSet = clusterClientSet

	o.Version = os.Getenv("VERSION")
	if o.Version == "" {
		o.Version = version.GetReleaseVersion().PatchRelease()
	}

	return nil
}

// Complete preparation for option
func (o *AddonOption) Complete(opts *cmdOptions.Options) error {
	return o.buildClusterConfig(opts)
}

// GetSpecNamespace return spec.namespace
func (o *AddonOption) GetSpecNamespace() string {
	return o.Spec.Namespace
}

func (o *AddonOption) GetImageRepository() string {
	return o.Spec.ClusterLinkOptions.ImageRepository
}

func (o *AddonOption) GetIPFamily() string {
	return string(o.Spec.ClusterLinkOptions.IPFamily)
}
