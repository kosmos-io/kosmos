package tasks

import (
	"k8s.io/client-go/dynamic"
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
	ExternalIP() string
	HostPort() int32
	HostPortMap() map[string]int32
	DynamicClient() *dynamic.DynamicClient
}
