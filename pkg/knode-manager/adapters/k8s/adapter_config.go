package k8sadapter

import (
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/knode-manager/utils/manager"
)

type AdapterConfig struct {
	Client kubernetes.Interface
	Master kubernetes.Interface

	PodInformer                   v1.PodInformer
	NamespaceInformer             v1.NamespaceInformer
	NodeInformer                  v1.NodeInformer
	ConfigmapInformer             v1.ConfigMapInformer
	SecretInformer                v1.SecretInformer
	ServiceInformer               v1.ServiceInformer
	PersistentVolumeInformer      v1.PersistentVolumeInformer
	PersistentVolumeClaimInformer v1.PersistentVolumeClaimInformer

	ResourceManager *manager.ResourceManager
}
