package options

import (
	"github.com/spf13/pflag"
)

type Options struct {
	KubeConfig             string
	ControlPanelKubeConfig string
	ExternalKubeConfigName string
	UseProxy               bool
}

// NewOptions builds a default agent options.
func NewOptions() *Options {
	return &Options{
		KubeConfig: "",
	}
}

// AddFlags adds flags of estimator to the specified FlagSet
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}
	fs.StringVar(&o.KubeConfig, "kubeconfig", "", "Path to control plane kubeconfig file.")
	fs.StringVar(&o.ControlPanelKubeConfig, "controlpanelconfig", "", "Path to host control plane kubeconfig file.")
	fs.StringVar(&o.ExternalKubeConfigName, "ExternalKubeConfigName", "external-kubeconfig", "external kube config name.")
	fs.BoolVar(&o.UseProxy, "UseProxy", false, "external kube config name.")
}
