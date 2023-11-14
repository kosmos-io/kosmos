package options

import (
	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/config/options"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

const (
	LeaderElectionNamespace    = "kosmos-system"
	LeaderElectionResourceName = "cluster-manager"

	DefaultKubeQPS   = 40.0
	DefaultKubeBurst = 60

	CoreDNSServiceNamespace = "kube-system"
	CoreDNSServiceName      = "kube-dns"
)

type Options struct {
	LeaderElection      componentbaseconfig.LeaderElectionConfiguration
	KubernetesOptions   KubernetesOptions
	ListenPort          int32
	DaemonSetController bool
	MultiClusterService bool

	// If MultiClusterService is disabled, the clustertree will rewrite the dnsPolicy configuration for pods deployed in
	// the leaf clusters, directing them to the root cluster's CoreDNS, thus facilitating access to services across all
	// clusters.
	RootCoreDNSServiceNamespace string
	RootCoreDNSServiceName      string

	HTTPListenAddr string
}

type KubernetesOptions struct {
	KubeConfig string  `json:"kubeconfig" yaml:"kubeconfig"`
	Master     string  `json:"master,omitempty" yaml:"master,omitempty"`
	QPS        float32 `json:"qps,omitempty" yaml:"qps,omitempty"`
	Burst      int     `json:"burst,omitempty" yaml:"burst,omitempty"`
}

func NewOptions() (*Options, error) {
	var leaderElection componentbaseconfigv1alpha1.LeaderElectionConfiguration
	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(&leaderElection)

	leaderElection.ResourceName = LeaderElectionResourceName
	leaderElection.ResourceNamespace = LeaderElectionNamespace
	leaderElection.ResourceLock = resourcelock.LeasesResourceLock

	var opts Options
	if err := componentbaseconfigv1alpha1.Convert_v1alpha1_LeaderElectionConfiguration_To_config_LeaderElectionConfiguration(&leaderElection, &opts.LeaderElection, nil); err != nil {
		return nil, err
	}

	return &opts, nil
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	if o == nil {
		return
	}

	flags.Float32Var(&o.KubernetesOptions.QPS, "kube-qps", DefaultKubeQPS, "QPS to use while talking with kube-apiserver.")
	flags.IntVar(&o.KubernetesOptions.Burst, "kube-burst", DefaultKubeBurst, "Burst to use while talking with kube-apiserver.")
	flags.StringVar(&o.KubernetesOptions.KubeConfig, "kubeconfig", "", "Path for kubernetes kubeconfig file, if left blank, will use in cluster way.")
	flags.StringVar(&o.KubernetesOptions.Master, "master", "", "Used to generate kubeconfig for downloading, if not specified, will use host in kubeconfig.")
	flags.Int32Var(&o.ListenPort, "listen-port", 10250, "Listen port for requests from the kube-apiserver.")
	flags.BoolVar(&o.DaemonSetController, "daemonset-controller", false, "Turn on or off daemonset controller.")
	flags.BoolVar(&o.MultiClusterService, "multi-cluster-service", false, "Turn on or off mcs support.")
	flags.StringVar(&o.RootCoreDNSServiceNamespace, "root-coredns-service-namespace", CoreDNSServiceNamespace, "The namespace of the CoreDNS service in the root cluster, used to locate the CoreDNS service when MultiClusterService is disabled.")
	flags.StringVar(&o.RootCoreDNSServiceName, "root-coredns-service-name", CoreDNSServiceName, "The name of the CoreDNS service in the root cluster, used to locate the CoreDNS service when MultiClusterService is disabled.")

	flags.StringVar(&o.HTTPListenAddr, "http-listen-addr", ":10250", "Http listen addr, default is :10250.")

	options.BindLeaderElectionFlags(&o.LeaderElection, flags)
}
