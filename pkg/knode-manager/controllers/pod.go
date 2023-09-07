package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	"github.com/kosmos.io/kosmos/pkg/knode-manager/adapters"
)

type PodConfig struct {
	PodClient corev1client.PodsGetter

	PodInformer corev1informers.PodInformer

	EventRecorder record.EventRecorder

	PodHandler adapters.PodHandler

	ConfigMapInformer corev1informers.ConfigMapInformer
	SecretInformer    corev1informers.SecretInformer
	ServiceInformer   corev1informers.ServiceInformer
}

type PodController struct {
	podHandler adapters.PodHandler

	podsInformer corev1informers.PodInformer

	podsLister corev1listers.PodLister

	// nolint:unused
	recorder record.EventRecorder

	client corev1client.PodsGetter
}

func NewPodController(cfg PodConfig) (*PodController, error) {
	if cfg.PodClient == nil {
		return nil, fmt.Errorf("missing core client")
	}
	if cfg.EventRecorder == nil {
		return nil, fmt.Errorf("missing event recorder")
	}
	if cfg.PodInformer == nil {
		return nil, fmt.Errorf("missing pod informer")
	}
	if cfg.ConfigMapInformer == nil {
		return nil, fmt.Errorf("missing config map informer")
	}
	if cfg.SecretInformer == nil {
		return nil, fmt.Errorf("missing secret informer")
	}
	if cfg.ServiceInformer == nil {
		return nil, fmt.Errorf("missing service informer")
	}
	if cfg.PodHandler == nil {
		return nil, fmt.Errorf("missing podHandler")
	}

	pc := &PodController{
		client:       cfg.PodClient,
		podsInformer: cfg.PodInformer,
		podsLister:   cfg.PodInformer.Lister(),
		podHandler:   cfg.PodHandler,
	}

	return pc, nil
}

func (pd *PodController) Run(ctx context.Context, podSyncWorkers int) (retErr error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var eventHandler cache.ResourceEventHandler = cache.ResourceEventHandlerFuncs{
		AddFunc: func(pod interface{}) {
			// nolint:errcheck
			pd.podHandler.Create(ctx, &corev1.Pod{})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// nolint:errcheck
			pd.podHandler.Update(ctx, &corev1.Pod{})
		},
		DeleteFunc: func(pod interface{}) {
			// nolint:errcheck
			pd.podHandler.Delete(ctx, &corev1.Pod{})
		},
	}

	if _, err := pd.podsInformer.Informer().AddEventHandler(eventHandler); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}
