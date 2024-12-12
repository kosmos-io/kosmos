package options

import (
	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

type Options struct {
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
	utils.KubernetesOptions
}

func NewOptions() *Options {
	return &Options{
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: utils.DefaultNamespace,
			ResourceName:      "network-manager",
		},
	}
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	if o == nil {
		return
	}

	flags.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", true, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	flags.StringVar(&o.LeaderElection.ResourceName, "leader-elect-resource-name", "network-manager", "The name of resource object that is used for locking during leader election.")
	flags.StringVar(&o.LeaderElection.ResourceNamespace, "leader-elect-resource-namespace", utils.DefaultNamespace, "The namespace of resource object that is used for locking during leader election.")
	flags.Float32Var(&o.KubernetesOptions.QPS, "kube-qps", utils.DefaultTreeAndNetManagerKubeQPS, "QPS to use while talking with kube-apiserver.")
	flags.IntVar(&o.KubernetesOptions.Burst, "kube-burst", utils.DefaultTreeAndNetManagerKubeBurst, "Burst to use while talking with kube-apiserver.")
	flags.StringVar(&o.KubernetesOptions.KubeConfig, "kubeconfig", "", "Path to host kubeconfig file.")
	flags.StringVar(&o.KubernetesOptions.MasterURL, "master-url", "", "Used to generate kubeconfig for downloading, if not specified, will use host in kubeconfig.")
}
