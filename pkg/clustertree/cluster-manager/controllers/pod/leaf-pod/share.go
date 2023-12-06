package leafpodsyncers

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
)

type LeafPodWorkerQueueOption struct {
	Config           *rest.Config
	RootClient       kubernetes.Interface
	ServerlessClient *utils.ServerlessClient
}
