package app

import (
	"context"
	"time"

	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/cmd/clustertree/knode-manager/app/config"
	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/extensions/daemonset"
	"github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	"github.com/kosmos.io/kosmos/pkg/utils/flags"
)

// StartHostDaemonSetsController starts a new HostDaemonSetsController.
func StartHostDaemonSetsController(ctx context.Context, c *config.Config, workNum int) (*daemonset.HostDaemonSetsController, error) {
	kubeClient, err := clientset.NewForConfig(c.KubeConfig)
	if err != nil {
		klog.V(2).Infof("Unable to create kubeClient: %v", err)
		return nil, err
	}
	kubeFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	kosmosFactory := externalversions.NewSharedInformerFactory(c.CRDClient, 0)

	controller, err := daemonset.NewHostDaemonSetsController(
		kosmosFactory.Kosmos().V1alpha1().ShadowDaemonSets(),
		kubeFactory.Apps().V1().ControllerRevisions(),
		kubeFactory.Core().V1().Pods(),
		kubeFactory.Core().V1().Nodes(),
		c.CRDClient,
		kubeClient,
		flowcontrol.NewBackOff(1*time.Second, 15*time.Minute),
	)
	kubeFactory.Start(ctx.Done())
	kosmosFactory.Start(ctx.Done())
	if err != nil {
		return nil, err
	}
	go controller.Run(ctx, workNum)
	return controller, nil
}

func StartDistributeController(ctx context.Context, c *config.Config, workNum int) (*daemonset.DistributeController, error) {
	kosmosFactory := externalversions.NewSharedInformerFactory(c.CRDClient, 0)
	option := flags.Options{}
	controller := daemonset.NewDistributeController(
		c.CRDClient,
		kosmosFactory.Kosmos().V1alpha1().ShadowDaemonSets(),
		kosmosFactory.Kosmos().V1alpha1().Knodes(),
		option,
	)
	kosmosFactory.Start(ctx.Done())

	go controller.Run(ctx, workNum)

	return controller, nil
}

func StartDaemonSetsController(ctx context.Context, c *config.Config, workNum int) (*daemonset.DaemonSetsController, error) {
	kubeClient, err := clientset.NewForConfig(c.KubeConfig)
	if err != nil {
		klog.V(2).Infof("Unable to create kubeClient: %v", err)
		return nil, err
	}

	kosmosFactory := externalversions.NewSharedInformerFactory(c.CRDClient, 0)
	option := flags.Options{}
	controller := daemonset.NewDaemonSetsController(
		kosmosFactory.Kosmos().V1alpha1().ShadowDaemonSets(),
		kosmosFactory.Kosmos().V1alpha1().DaemonSets(),
		kosmosFactory.Kosmos().V1alpha1().Knodes(),
		kubeClient,
		c.CRDClient,
		kosmosFactory.Kosmos().V1alpha1().DaemonSets().Lister(),
		kosmosFactory.Kosmos().V1alpha1().ShadowDaemonSets().Lister(),
		kosmosFactory.Kosmos().V1alpha1().Knodes().Lister(),
		option,
	)
	kosmosFactory.Start(ctx.Done())

	go controller.Run(ctx, workNum)

	return controller, nil
}

func StartDaemonSetsMirrorController(ctx context.Context, c *config.Config, workNum int) (*daemonset.DaemonSetsMirrorController, error) {
	kubeClient, err := clientset.NewForConfig(c.KubeConfig)
	if err != nil {
		klog.V(2).Infof("Unable to create kubeClient: %v", err)
		return nil, err
	}
	kosmosFactory := externalversions.NewSharedInformerFactory(c.CRDClient, 0)
	kubeFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	option := flags.Options{}
	controller := daemonset.NewDaemonSetsMirrorController(
		c.CRDClient,
		kubeClient,
		kosmosFactory.Kosmos().V1alpha1().DaemonSets(),
		kubeFactory.Apps().V1().DaemonSets(),
		option,
	)
	kosmosFactory.Start(ctx.Done())
	kubeFactory.Start(ctx.Done())
	go controller.Run(ctx, workNum)
	return controller, nil
}
