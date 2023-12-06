package pod

import (
	leafpodsyncers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod/leaf-pod"
	k8s "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod/leaf-pod/k8s"
	serverless "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod/leaf-pod/serverless"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/runtime"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
)

func NewLeafPodWorkerQueue(opts *leafpodsyncers.LeafPodWorkerQueueOption, leafType leafUtils.LeafType) runtime.Controller {
	switch leafType {
	case leafUtils.LeafTypeK8s:
		return k8s.NewLeafPodK8wWorkerQueue(opts)
	case leafUtils.LeafTypeServerless:
		return serverless.NewLeafPodOpenApiWorkerQueue(opts)
	default:
		panic("leaf type not supported")
	}
	// return runtime.Controller{}
}
