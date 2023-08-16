package option

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	cmdOptions "github.com/kosmos.io/clusterlink/cmd/operator/app/options"
	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/version"
)

// AddonOption for cluster
type AddonOption struct {
	clusterlinkv1alpha1.Cluster
	KubeClientSet          *kubernetes.Clientset
	ControlPanelKubeConfig *clientcmdapi.Config
	Version                string
	UseProxy               bool
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

// preparation for option
func (o *AddonOption) Complete(opts *cmdOptions.Options) error {
	return o.buildClusterConfig(opts)
}

// return spec.namespace
func (o *AddonOption) GetSpecNamespace() string {
	return o.Spec.Namespace
}

func (o *AddonOption) GetImageRepository() string {
	return o.Spec.ImageRepository
}

func (o *AddonOption) GetIPFamily() string {
	return string(o.Spec.IPFamily)
}
