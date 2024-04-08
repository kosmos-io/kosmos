package tasks

import (
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util/cert"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type InitData interface {
	cert.CertStore
	GetName() string
	GetNamespace() string
	SetControlplaneConfig(config *rest.Config)
	ControlplaneConfig() *rest.Config
	ControlplaneAddress() string
	ServiceClusterIp() []string
	RemoteClient() clientset.Interface
	VirtualClusterClient() clientset.Interface
	KosmosClient() versioned.Interface
	DataDir() string
	VirtualClusterVersion() string
}
