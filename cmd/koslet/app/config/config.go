package config

import (
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	componentbaseconfig "k8s.io/component-base/config"

	crdclientset "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

type Config struct {
	Client         *clientset.Clientset
	KubeConfig     *restclient.Config
	CRDClient      *crdclientset.Clientset
	WorkerNumber   int
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
	HomeName       string
	KubeAPIQPS     float32
	KubeAPIBurst   int
	KubeConfigPath string
}
