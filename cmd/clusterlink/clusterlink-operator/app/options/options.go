package options

import (
	"github.com/spf13/pflag"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

type Options struct {
	KubernetesOptions      utils.KubernetesOptions
	ExternalKubeConfigName string
	UseProxy               bool
}

// NewOptions builds a default agent options.
func NewOptions() *Options {
	return &Options{}
}

// AddFlags adds flags of estimator to the specified FlagSet
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}

	fs.Float32Var(&o.KubernetesOptions.QPS, "kube-qps", utils.DefaultKubeQPS, "QPS to use while talking with kube-apiserver.")
	fs.IntVar(&o.KubernetesOptions.Burst, "kube-burst", utils.DefaultKubeBurst, "Burst to use while talking with kube-apiserver.")
	fs.StringVar(&o.KubernetesOptions.KubeConfig, "kubeconfig", "", "Path to host kubeconfig file.")
	fs.StringVar(&o.KubernetesOptions.MasterURL, "master-url", "", "Used to generate kubeconfig for downloading, if not specified, will use host in kubeconfig.")
	fs.StringVar(&o.KubernetesOptions.ControlpanelKubeConfig, "controlpanel-kubeconfig", "", "Path to control plane kubeconfig file.")
	fs.StringVar(&o.KubernetesOptions.ControlpanelMasterURL, "controlpanel-master-url", "", "Used to generate host control plane kubeconfig for downloading, if not specified, will use host in controlpanel-kubeconfig.")
	fs.StringVar(&o.ExternalKubeConfigName, "ExternalKubeConfigName", "external-kubeconfig", "external kube config name.")
	fs.BoolVar(&o.UseProxy, "UseProxy", false, "external kube config name.")
}
