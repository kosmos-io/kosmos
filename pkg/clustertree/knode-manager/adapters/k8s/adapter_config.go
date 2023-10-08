package k8sadapter

import (
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/utils/manager"
)

type AdapterConfig struct {
	Client kubernetes.Interface
	Master kubernetes.Interface

	PodInformer       v1.PodInformer
	NamespaceInformer v1.NamespaceInformer
	NodeInformer      v1.NodeInformer
	ConfigmapInformer v1.ConfigMapInformer
	SecretInformer    v1.SecretInformer
	ServiceInformer   v1.ServiceInformer

	ResourceManager *manager.ResourceManager
}
