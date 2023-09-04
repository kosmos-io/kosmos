package config

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	restclient "k8s.io/client-go/rest"
	componentbaseconfig "k8s.io/component-base/config"

	crdclientset "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

const (
	DefaultNodeName             = "kosmos-Node"
	DefaultInformerResyncPeriod = 1 * time.Minute
	DefaultListenPort           = 10250
	DefaultPodSyncWorkers       = 10
	DefaultKubeNamespace        = corev1.NamespaceAll
	DefaultKubeClusterDomain    = "cluster.local"

	DefaultTaintEffect = string(corev1.TaintEffectNoSchedule)
	DefaultTaintKey    = "kosmos-node.io/plugin"
)

type Config struct {
	KubeConfig     *restclient.Config
	CRDClient      *crdclientset.Clientset
	WorkerNumber   int
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
	Opts           *Opts
}

type Opts struct {
	KubeConfigPath    string
	KubeNamespace     string
	KubeClusterDomain string

	ListenPort int32

	NodeName string

	Plugin           string
	PluginConfigPath string

	TaintKey     string
	TaintEffect  string
	TaintValue   string
	DisableTaint bool

	PodSyncWorkers       int
	InformerResyncPeriod time.Duration

	EnableNodeLease bool

	StartupTimeout time.Duration

	KubeAPIQPS   int32
	KubeAPIBurst int32

	Version string
}

func New() *Opts {
	o := &Opts{}
	setDefaults(o)
	return o
}

func FromDefault() (*Opts, error) {
	o := &Opts{}
	setDefaults(o)

	return o, nil
}

func setDefaults(o *Opts) {
	o.NodeName = DefaultNodeName
	o.TaintKey = DefaultTaintKey
	o.TaintEffect = DefaultTaintEffect
	o.KubeNamespace = DefaultKubeNamespace
	o.PodSyncWorkers = DefaultPodSyncWorkers
	o.ListenPort = DefaultListenPort
	o.InformerResyncPeriod = DefaultInformerResyncPeriod
	o.KubeClusterDomain = DefaultKubeClusterDomain
	o.EnableNodeLease = true
}
