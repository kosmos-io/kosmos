package config

import (
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	componentbaseconfig "k8s.io/component-base/config"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

// Config has all the configurations for kubenest.
type Config struct {
	KubeNestOptions  v1alpha1.KubeNestConfiguration
	Client           clientset.Interface
	RestConfig       *restclient.Config
	KubeconfigStream []byte
	// LeaderElection is optional.
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
	// Core namespaces of KubeNest in vc cluster
	CoreNamespaces []string
	FeatureGates   []string
}
