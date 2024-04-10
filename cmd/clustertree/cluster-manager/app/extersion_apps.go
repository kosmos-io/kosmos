package app

import (
	"context"
	"time"

	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/options"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/extensions/daemonset"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
)

// StartHostDaemonSetsController starts a new HostDaemonSetsController.
func StartHostDaemonSetsController(ctx context.Context, opts *options.Options, workNum int) (*daemonset.HostDaemonSetsController, error) {
	kubeconfig, err := clientcmd.BuildConfigFromFlags(opts.KubernetesOptions.Master, opts.KubernetesOptions.KubeConfig)
	if err != nil {
		klog.Errorf("Unable to build kubeconfig: %v", err)
	}
	kubeClient, err := clientset.NewForConfig(kubeconfig)
	if err != nil {
		klog.Errorf("Unable to create kubeClient: %v", err)
		return nil, err
	}
	kosmosClient, err := versioned.NewForConfig(kubeconfig)
	if err != nil {
		klog.Errorf("Unable to create kosmosClient: %v", err)
	}

	kubeFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	kosmosFactory := externalversions.NewSharedInformerFactory(kosmosClient, 0)

	controller, err := daemonset.NewHostDaemonSetsController(
		kosmosFactory.Kosmos().V1alpha1().ShadowDaemonSets(),
		kubeFactory.Apps().V1().ControllerRevisions(),
		kubeFactory.Core().V1().Pods(),
		kubeFactory.Core().V1().Nodes(),
		kosmosClient,
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

func StartDistributeController(ctx context.Context, opts *options.Options, workNum int) (*daemonset.DistributeController, error) {
	kubeconfig, err := clientcmd.BuildConfigFromFlags(opts.KubernetesOptions.Master, opts.KubernetesOptions.KubeConfig)
	if err != nil {
		//klog.Errorf("Unable to build kubeconfig: %v", err)
		klog.Errorf("Unable to build kubeconfig: %v", err)
	}
	kosmosClient, err := versioned.NewForConfig(kubeconfig)
	if err != nil {
		klog.Errorf("Unable to create kosmosClient: %v", err)
	}

	kosmosFactory := externalversions.NewSharedInformerFactory(kosmosClient, 0)
	option := lifted.RateLimitOptions{}
	controller := daemonset.NewDistributeController(
		kosmosClient,
		kosmosFactory.Kosmos().V1alpha1().ShadowDaemonSets(),
		kosmosFactory.Kosmos().V1alpha1().Clusters(),
		option,
	)
	kosmosFactory.Start(ctx.Done())

	controller.Run(ctx, workNum)

	return controller, nil
}

func StartDaemonSetsController(ctx context.Context, opts *options.Options, workNum int) (*daemonset.DaemonSetsController, error) {
	kubeconfig, err := clientcmd.BuildConfigFromFlags(opts.KubernetesOptions.Master, opts.KubernetesOptions.KubeConfig)
	if err != nil {
		klog.Errorf("Unable to build kubeconfig: %v", err)
	}
	kubeClient, err := clientset.NewForConfig(kubeconfig)
	if err != nil {
		klog.Errorf("Unable to create kubeClient: %v", err)
		return nil, err
	}
	kosmosClient, err := versioned.NewForConfig(kubeconfig)
	if err != nil {
		klog.Errorf("Unable to create kosmosClient: %v", err)
		return nil, err
	}

	kosmosFactory := externalversions.NewSharedInformerFactory(kosmosClient, 0)
	option := lifted.RateLimitOptions{}
	controller := daemonset.NewDaemonSetsController(
		kosmosFactory.Kosmos().V1alpha1().ShadowDaemonSets(),
		kosmosFactory.Kosmos().V1alpha1().DaemonSets(),
		kosmosFactory.Kosmos().V1alpha1().Clusters(),
		kubeClient,
		kosmosClient,
		option,
	)
	kosmosFactory.Start(ctx.Done())

	controller.Run(ctx, workNum)

	return controller, nil
}

func StartDaemonSetsMirrorController(ctx context.Context, opts *options.Options, workNum int) (*daemonset.DaemonSetsMirrorController, error) {
	kubeconfig, err := clientcmd.BuildConfigFromFlags(opts.KubernetesOptions.Master, opts.KubernetesOptions.KubeConfig)
	if err != nil {
		klog.Errorf("Unable to build kubeconfig: %v", err)
	}
	kubeClient, err := clientset.NewForConfig(kubeconfig)
	if err != nil {
		klog.Errorf("Unable to create kubeClient: %v", err)
		return nil, err
	}
	kosmosClient, err := versioned.NewForConfig(kubeconfig)
	if err != nil {
		klog.Errorf("Unable to create kosmosClient: %v", err)
		return nil, err
	}
	kosmosFactory := externalversions.NewSharedInformerFactory(kosmosClient, 0)
	kubeFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	option := lifted.RateLimitOptions{}
	controller := daemonset.NewDaemonSetsMirrorController(
		kosmosClient,
		kubeClient,
		kosmosFactory.Kosmos().V1alpha1().DaemonSets(),
		kubeFactory.Apps().V1().DaemonSets(),
		option,
	)
	kosmosFactory.Start(ctx.Done())
	kubeFactory.Start(ctx.Done())
	controller.Run(ctx, workNum)
	return controller, nil
}

func StartPodReflectController(ctx context.Context, opts *options.Options, workNum int) (*daemonset.PodReflectorController, error) {
	kubeconfig, err := clientcmd.BuildConfigFromFlags(opts.KubernetesOptions.Master, opts.KubernetesOptions.KubeConfig)
	if err != nil {
		klog.Errorf("Unable to build kubeconfig: %v", err)
	}
	kubeClient, err := clientset.NewForConfig(kubeconfig)
	if err != nil {
		klog.Errorf("Unable to create kubeClient: %v", err)
		return nil, err
	}
	kosmosClient, err := versioned.NewForConfig(kubeconfig)
	if err != nil {
		klog.Errorf("Unable to create kosmosClient: %v", err)
		return nil, err
	}
	kosmosFactory := externalversions.NewSharedInformerFactory(kosmosClient, 0)
	kubeFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	option := lifted.RateLimitOptions{}
	controller := daemonset.NewPodReflectorController(
		kubeClient,
		kubeFactory.Apps().V1().DaemonSets(),
		kosmosFactory.Kosmos().V1alpha1().DaemonSets(),
		kosmosFactory.Kosmos().V1alpha1().Clusters(),
		kubeFactory.Core().V1().Pods(),
		option,
	)
	kosmosFactory.Start(ctx.Done())
	kubeFactory.Start(ctx.Done())
	controller.Run(ctx, workNum)
	return controller, nil
}

type GlobalDaemonSetService struct {
	ctx            context.Context
	opts           *options.Options
	defaultWorkNum int
}

func (g *GlobalDaemonSetService) SetupWithManager(mgr manager.Manager) error {
	return mgr.Add(g)
}

func (g *GlobalDaemonSetService) Start(context.Context) error {
	return enableGlobalDaemonSet(g.ctx, g.opts, g.defaultWorkNum)
}

func enableGlobalDaemonSet(ctx context.Context, opts *options.Options, defaultWorkNum int) error {
	_, err := StartHostDaemonSetsController(ctx, opts, defaultWorkNum)
	if err != nil {
		klog.Errorf("start host daemonset controller failed: %v", err)
		return err
	}
	_, err = StartDaemonSetsController(ctx, opts, defaultWorkNum)
	if err != nil {
		klog.Errorf("start daemon set controller failed: %v", err)
		return err
	}
	_, err = StartDistributeController(ctx, opts, defaultWorkNum)
	if err != nil {
		klog.Errorf("start distribute controller failed: %v", err)
		return err
	}
	_, err = StartDaemonSetsMirrorController(ctx, opts, defaultWorkNum)
	if err != nil {
		klog.Errorf("start daemon set mirror controller failed: %v", err)
		return err
	}

	_, err = StartPodReflectController(ctx, opts, defaultWorkNum)
	if err != nil {
		klog.Errorf("start pod reflect controller failed: %v", err)
		return err
	}
	return nil
}
