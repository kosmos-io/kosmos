package tasks

import (
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kosmos.io/kosmos/pkg/treeoperator/cert"
)

type InitData interface {
	cert.CertStore
	GetName() string
	GetNamespace() string
	SetControlplaneConfig(config *rest.Config)
	ControlplaneConfig() *rest.Config
	ControlplaneAddress() string
	RemoteClient() clientset.Interface
	VirtualClusterClient() clientset.Interface
	KosmosClient() versioned.Interface
	DataDir() string
	VirtualClusterVersion() string
}
