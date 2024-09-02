package config

import (
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	componentbaseconfig "k8s.io/component-base/config"
)

// Config has all the configurations for kubenest.
type Config struct {
	Client         clientset.Interface
	RestConfig     *restclient.Config
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
}
