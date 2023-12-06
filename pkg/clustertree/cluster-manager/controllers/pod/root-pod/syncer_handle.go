package rootpodsyncers

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/runtime"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
)

type RootPodSyncerHandle interface {
	DeletePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootnamespacedname runtime.NamespacedName, cleanflag bool) error
	CreatePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, pod *corev1.Pod) error
	UpdatePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootpod *corev1.Pod, leafpod *corev1.Pod) error
	GetPodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootnamespacedname runtime.NamespacedName) (*corev1.Pod, error)
}
