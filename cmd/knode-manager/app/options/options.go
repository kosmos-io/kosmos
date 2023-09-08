package options

import (
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	componentbaseconfig "k8s.io/component-base/config"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	"github.com/kosmos.io/kosmos/cmd/knode-manager/app/config"
	crdclientset "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

const (
	KosmosNodeManagerUserAgent = "kosmos-node-manager"
)

var (
	buildVersion    = "N/A"
	numberOfWorkers = 50
)

type Options struct {
	LeaderElection componentbaseconfig.LeaderElectionConfiguration

	WorkerNumber int

	Opts *config.Opts
}

func NewKosmosNodeOptions() (*Options, error) {
	var leaderElection componentbaseconfigv1alpha1.LeaderElectionConfiguration
	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(&leaderElection)

	leaderElection.ResourceName = "kosmosnode-manager"
	leaderElection.ResourceNamespace = "kosmosnode-system"
	leaderElection.ResourceLock = resourcelock.LeasesResourceLock

	var options Options
	if err := componentbaseconfigv1alpha1.Convert_v1alpha1_LeaderElectionConfiguration_To_config_LeaderElectionConfiguration(&leaderElection, &options.LeaderElection, nil); err != nil {
		return nil, err
	}

	options.WorkerNumber = 1

	o, err := config.FromDefault()
	if err != nil {
		panic(err)
	}
	o.PodSyncWorkers = numberOfWorkers
	o.Version = buildVersion

	options.Opts = o

	return &options, nil
}

func (o *Options) Config() (*config.Config, error) {
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", o.Opts.KubeConfigPath)
	if err != nil {
		return nil, err
	}

	crdclient, err := crdclientset.NewForConfig(restclient.AddUserAgent(kubeconfig, KosmosNodeManagerUserAgent))
	if err != nil {
		return nil, err
	}

	return &config.Config{
		WorkerNumber:   o.WorkerNumber,
		LeaderElection: o.LeaderElection,
		KubeConfig:     kubeconfig,
		CRDClient:      crdclient,
		Opts:           o.Opts,
	}, nil
}

func (o *Options) Flags() cliflag.NamedFlagSets {
	var fss cliflag.NamedFlagSets

	fs := fss.FlagSet("misc")
	fs.StringVar(&o.Opts.KubeConfigPath, "kubeconfig", o.Opts.KubeConfigPath, "kube config file to use for connecting to the Kubernetes API server")
	fs.StringVar(&o.Opts.KubeNamespace, "namespace", o.Opts.KubeNamespace, "kubernetes namespace (default is 'all')")

	fs.DurationVar(&o.Opts.InformerResyncPeriod, "full-resync-period", o.Opts.InformerResyncPeriod, "how often to perform a full resync of pods between kubernetes and knode")
	fs.DurationVar(&o.Opts.StartupTimeout, "startup-timeout", o.Opts.StartupTimeout, "How long to wait for the cluster-router to start")

	fs.IntVar(&o.Opts.PodSyncWorkers, "pod-sync-workers", o.Opts.PodSyncWorkers, `set the number of pod synchronization workers`)
	fs.BoolVar(&o.Opts.EnableNodeLease, "enable-node-lease", o.Opts.EnableNodeLease, `use node leases (1.13) for node heartbeats`)

	fs.Float32Var(&o.Opts.KubeAPIQPS, "kube-api-qps", o.Opts.KubeAPIQPS,
		"kubeAPIQPS is the QPS to use while talking with kubernetes apiserver")
	fs.IntVar(&o.Opts.KubeAPIBurst, "kube-api-burst", o.Opts.KubeAPIBurst,
		"kubeAPIBurst is the burst to allow while talking with kubernetes apiserver")

	fs.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", o.LeaderElection.LeaderElect, ""+
		"Start a leader election client and gain leadership before "+
		"executing the main loop. Enable this when running replicated "+
		"components for high availability.")
	return fss
}
