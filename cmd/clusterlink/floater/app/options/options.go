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
	return &Options{}
}

// AddFlags adds flags of agent to the specified FlagSet
func (o *Options) AddFlags(_ *pflag.FlagSet) {
}
