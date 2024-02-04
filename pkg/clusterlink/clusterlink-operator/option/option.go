package option

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/version"
)

// AddonOption for cluster
type AddonOption struct {
	kosmosv1alpha1.Cluster

	Version  string
	UseProxy bool

	KubeConfigByte         []byte
	KubeClientSet          *kubernetes.Clientset
	ControlPanelKubeConfig *clientcmdapi.Config
}

func (o *AddonOption) buildClusterConfig() error {
	restConfig, err := utils.NewConfigFromBytes(o.KubeConfigByte)
	if err != nil {
		return fmt.Errorf("error building restConfig: %s", err.Error())
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
func (o *AddonOption) Complete() error {
	return o.buildClusterConfig()
}

// GetSpecNamespace return spec.namespace
func (o *AddonOption) GetSpecNamespace() string {
	return o.Spec.Namespace
}

func (o *AddonOption) GetImageRepository() string {
	return o.Spec.ImageRepository
}

func (o *AddonOption) GetIPFamily() string {
	return string(o.Spec.ClusterLinkOptions.IPFamily)
}
