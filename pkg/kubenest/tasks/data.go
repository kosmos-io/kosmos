package tasks

import (
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
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
	VirtualCluster() *v1alpha1.VirtualCluster
	ExternalIP() string
	ExternalIPs() []string
	HostPort() int32
	HostPortMap() map[string]int32
	VipMap() map[string]string
	DynamicClient() *dynamic.DynamicClient
	KubeNestOpt() *v1alpha1.KubeNestConfiguration
	PluginOptions() map[string]string
}
