package tasks

import (
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util/cert"
	clientset "k8s.io/client-go/kubernetes"

	vcnodecontroller "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller"
)

type InitData interface {
	cert.CertStore
	GetName() string
	GetNamespace() string
	GetHostPortManager() *vcnodecontroller.HostPortManager
	ControlplaneAddress() string
	ServiceClusterIp() []string
	RemoteClient() clientset.Interface
	KosmosClient() versioned.Interface
	DataDir() string
	VirtualClusterVersion() string
	ExternalIP() string
}
