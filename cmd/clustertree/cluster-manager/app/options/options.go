package options

import (
	"github.com/spf13/pflag"
	componentbaseconfig "k8s.io/component-base/config"
)

const (
	LeaderElectionNamespace    = "kosmos-system"
	LeaderElectionResourceName = "cluster-manager"

	DefaultKubeQPS   = 40.0
	DefaultKubeBurst = 60
)

type Options struct {
	LeaderElection    componentbaseconfig.LeaderElectionConfiguration
	KubernetesOptions KubernetesOptions
}

type KubernetesOptions struct {
	KubeConfig string  `json:"kubeconfig" yaml:"kubeconfig"`
	Master     string  `json:"master,omitempty" yaml:"master,omitempty"`
	QPS        float32 `json:"qps,omitempty" yaml:"qps,omitempty"`
	Burst      int     `json:"burst,omitempty" yaml:"burst,omitempty"`
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	if o == nil {
		return
	}

	flags.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", true, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	flags.StringVar(&o.LeaderElection.ResourceName, "leader-elect-resource-name", LeaderElectionResourceName, "The name of resource object that is used for locking during leader election.")
	flags.StringVar(&o.LeaderElection.ResourceNamespace, "leader-elect-resource-namespace", LeaderElectionNamespace, "The namespace of resource object that is used for locking during leader election.")
	flags.Float32Var(&o.KubernetesOptions.QPS, "kube-qps", DefaultKubeQPS, "QPS to use while talking with kube-apiserver.")
	flags.IntVar(&o.KubernetesOptions.Burst, "kube-burst", DefaultKubeBurst, "Burst to use while talking with kube-apiserver.")
	flags.StringVar(&o.KubernetesOptions.KubeConfig, "kubeconfig", "", "Path for kubernetes kubeconfig file, if left blank, will use in cluster way.")
	flags.StringVar(&o.KubernetesOptions.Master, "master", "", "Used to generate kubeconfig for downloading, if not specified, will use host in kubeconfig.")
}
