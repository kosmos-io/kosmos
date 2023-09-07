package knodemanager

import (
	"context"

	"github.com/pkg/errors"
	klogv2 "k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/cmd/knode-manager/app/config"
	k8sadapter "github.com/kosmos.io/kosmos/pkg/knode-manager/adapters/k8s"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/controllers"
)

type Knode struct {
	podController *controllers.PodController
}

func NewKnode(_ context.Context, _ *string, _ *config.Opts) (*Knode, error) {
	podHandler, err := k8sadapter.NewPodAdapter()

	if err != nil {
		return nil, err
	}

	kp, err := controllers.NewPodController(controllers.PodConfig{
		PodHandler: podHandler,
	})

	if err != nil {
		return nil, err
	}

	return &Knode{
		podController: kp,
	}, nil
}

func (kn *Knode) Run(ctx context.Context, c *config.Opts) {
	go func() {
		if err := kn.podController.Run(ctx, c.PodSyncWorkers); err != nil && errors.Cause(err) != context.Canceled {
			klogv2.Fatal(err)
		}
	}()
}
