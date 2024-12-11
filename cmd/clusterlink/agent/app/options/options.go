package options

import (
	"time"

	"github.com/spf13/pflag"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

type Options struct {
	KubernetesOptions utils.KubernetesOptions

	// CleanPeriod represents clusterlink-agent cleanup period
	CleanPeriod time.Duration
}

// NewOptions builds a default agent options.
func NewOptions() *Options {
	return &Options{}
}

// AddFlags adds flags of agent to the specified FlagSet
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}

	fs.Float32Var(&o.KubernetesOptions.QPS, "kube-qps", utils.DefaultKubeQPS, "QPS to use while talking with kube-apiserver.")
	fs.IntVar(&o.KubernetesOptions.Burst, "kube-burst", utils.DefaultKubeBurst, "Burst to use while talking with kube-apiserver.")
	fs.StringVar(&o.KubernetesOptions.KubeConfig, "kubeconfig", "", "Path to control plane kubeconfig file.")
	fs.StringVar(&o.KubernetesOptions.MasterURL, "master-url", "", "Used to generate kubeconfig for downloading, if not specified, will use host in control plane kubeconfig.")
	fs.DurationVar(&o.CleanPeriod, "clean-period", 30*time.Second, "Specifies how often the agent cleans up routes and network interface.")
}
