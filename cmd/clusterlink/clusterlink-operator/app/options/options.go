package options

import (
	"github.com/spf13/pflag"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

type Options struct {
	KubernetesOptions      utils.KubernetesOptions
	ControlPanelKubeConfig string
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
	fs.StringVar(&o.KubernetesOptions.KubeConfig, "kubeconfig", "", "Path to control plane kubeconfig file.")
	fs.StringVar(&o.ControlPanelKubeConfig, "controlpanelconfig", "", "Path to host control plane kubeconfig file.")
	fs.StringVar(&o.ExternalKubeConfigName, "ExternalKubeConfigName", "external-kubeconfig", "external kube config name.")
	fs.BoolVar(&o.UseProxy, "UseProxy", false, "external kube config name.")
}
