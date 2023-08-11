package options

import (
	"time"

	"github.com/spf13/pflag"
)

type Options struct {
	KubeConfig string

	// CleanPeriod represents clusterlink-agent cleanup period
	CleanPeriod time.Duration
}

// NewOptions builds a default agent options.
func NewOptions() *Options {
	return &Options{
		KubeConfig: "",
	}
}

// AddFlags adds flags of agent to the specified FlagSet
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}

	fs.StringVar(&o.KubeConfig, "kubeconfig", o.KubeConfig, "Path to control plane kubeconfig file.")
	fs.DurationVar(&o.CleanPeriod, "clean-period", 30*time.Second, "Specifies how often the agent cleans up routes and network interface.")
}
