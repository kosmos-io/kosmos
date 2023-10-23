package options

import (
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	componentbaseconfig "k8s.io/component-base/config"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	"github.com/kosmos.io/kosmos/cmd/koslet/app/config"
	crdclientset "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

const (
	KosletManagerUserAgent = "koslet-manager"
)

// Options hold the command-line options about kosmos koslet
type Options struct {
	LeaderElection componentbaseconfig.LeaderElectionConfiguration

	WorkerNumber int

	KubeConfigPath string

	HostnameOverride string

	KubeAPIQPS float32

	KubeAPIBurst int
}

func NewKosletOptions() (*Options, error) {
	var leaderElection componentbaseconfigv1alpha1.LeaderElectionConfiguration
	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(&leaderElection)

	leaderElection.ResourceName = "koslet-manager"
	leaderElection.ResourceNamespace = "kosmos-system"
	leaderElection.ResourceLock = resourcelock.LeasesResourceLock

	var options Options

	// not need scheme.Convert
	if err := componentbaseconfigv1alpha1.Convert_v1alpha1_LeaderElectionConfiguration_To_config_LeaderElectionConfiguration(&leaderElection, &options.LeaderElection, nil); err != nil {
		return nil, err
	}

	options.WorkerNumber = 1
	return &options, nil
}

func (o *Options) Config() (*config.Config, error) {
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", o.KubeConfigPath)
	if err != nil {
		return nil, err
	}

	crdclient, err := crdclientset.NewForConfig(restclient.AddUserAgent(kubeconfig, KosletManagerUserAgent))
	if err != nil {
		return nil, err
	}

	return &config.Config{
		WorkerNumber:   o.WorkerNumber,
		LeaderElection: o.LeaderElection,
		KubeConfig:     kubeconfig,
		CRDClient:      crdclient,
		HomeName:       o.HostnameOverride,
		KubeAPIQPS:     o.KubeAPIQPS,
		KubeAPIBurst:   o.KubeAPIBurst,
		KubeConfigPath: o.KubeConfigPath,
	}, nil
}

func (o *Options) Flags() cliflag.NamedFlagSets {
	var fss cliflag.NamedFlagSets

	fs := fss.FlagSet("misc")
	fs.StringVar(&o.KubeConfigPath, "kubeconfig", o.KubeConfigPath, "kube config file to use for connecting to the Kubernetes API server")
	fs.StringVar(&o.HostnameOverride, "hostname-override", "", "Which is the name of k8s node be used to filtered.")

	fs.Float32Var(&o.KubeAPIQPS, "kube-api-qps", o.KubeAPIQPS,
		"kubeAPIQPS is the QPS to use while talking with kubernetes apiserver")
	fs.IntVar(&o.KubeAPIBurst, "kube-api-burst", o.KubeAPIBurst,
		"kubeAPIBurst is the burst to allow while talking with kubernetes apiserver")

	fs.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", o.LeaderElection.LeaderElect, ""+
		"Start a leader election client and gain leadership before "+
		"executing the main loop. Enable this when running replicated "+
		"components for high availability.")
	return fss
}
