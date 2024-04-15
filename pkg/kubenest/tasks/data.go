package tasks

import (
	clientset "k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util/cert"
)

type InitData interface {
	cert.CertStore
	GetName() string
	GetNamespace() string
	ControlplaneAddress() string
	ServiceClusterIp() []string
	RemoteClient() clientset.Interface
	KosmosClient() versioned.Interface
	DataDir() string
	VirtualClusterVersion() string
}
