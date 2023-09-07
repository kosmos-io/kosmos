package k8sadapter

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/kosmos.io/kosmos/pkg/network"
)

type NodeAdapter struct {
}

func NewNodeAdapter() (*NodeAdapter, error) {
	return nil, network.ErrNotImplemented
}

func (n *NodeAdapter) Configure(ctx context.Context, node *corev1.Node) {
}

func (n *NodeAdapter) Trace(_ context.Context) error {
	return network.ErrNotImplemented
}

func (n *NodeAdapter) NotifyStatus(_ context.Context, _ func(*corev1.Node)) {
}
