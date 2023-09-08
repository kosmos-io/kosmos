package config

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	restclient "k8s.io/client-go/rest"
	componentbaseconfig "k8s.io/component-base/config"

	crdclientset "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

const (
	DefaultInformerResyncPeriod = 1 * time.Minute
	DefaultListenPort           = 10250
	DefaultPodSyncWorkers       = 10
	DefaultKubeNamespace        = corev1.NamespaceAll

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
	KubeConfigPath string
	KubeNamespace  string

	ListenPort int32

	TaintKey    string
	TaintEffect string
	TaintValue  string

	PodSyncWorkers       int
	InformerResyncPeriod time.Duration

	EnableNodeLease bool

	StartupTimeout time.Duration

	KubeAPIQPS   float32
	KubeAPIBurst int

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
	o.TaintKey = DefaultTaintKey
	o.TaintEffect = DefaultTaintEffect
	o.KubeNamespace = DefaultKubeNamespace
	o.PodSyncWorkers = DefaultPodSyncWorkers
	o.ListenPort = DefaultListenPort
	o.InformerResyncPeriod = DefaultInformerResyncPeriod
	o.EnableNodeLease = true
}
