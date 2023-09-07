package k8sadapter

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/kosmos.io/kosmos/pkg/network"
)

type PodAdapter struct {
}

func NewPodAdapter() (*PodAdapter, error) {
	return nil, network.ErrNotImplemented
}

func (p *PodAdapter) Create(ctx context.Context, pod *corev1.Pod) error {
	return network.ErrNotImplemented
}

func (p *PodAdapter) Update(ctx context.Context, pod *corev1.Pod) error {
	return network.ErrNotImplemented
}

func (p *PodAdapter) Delete(ctx context.Context, pod *corev1.Pod) error {
	return network.ErrNotImplemented
}

func (p *PodAdapter) Get(ctx context.Context, namespace string, name string) (*corev1.Pod, error) {
	return nil, network.ErrNotImplemented
}

func (p *PodAdapter) GetStatus(ctx context.Context, namespace string, name string) (*corev1.PodStatus, error) {
	return nil, network.ErrNotImplemented
}

func (p *PodAdapter) List(_ context.Context) ([]*corev1.Pod, error) {
	return nil, network.ErrNotImplemented
}

func (p *PodAdapter) Notify(ctx context.Context, f func(*corev1.Pod)) {

}
