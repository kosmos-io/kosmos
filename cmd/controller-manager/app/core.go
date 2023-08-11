package app

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/controllers/calicoippool"
	"github.com/kosmos.io/clusterlink/pkg/controllers/cluster"
	ctrlcontext "github.com/kosmos.io/clusterlink/pkg/controllers/context"
	"github.com/kosmos.io/clusterlink/pkg/controllers/node"
	"github.com/kosmos.io/clusterlink/pkg/controllers/nodecidr"
	"github.com/kosmos.io/clusterlink/pkg/generated/clientset/versioned"
)

func startClusterController(ctx ctrlcontext.Context) (bool, ctrlcontext.CleanFunc, error) {
	mgr := ctx.Mgr
	restConfig := mgr.GetConfig()

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("Unable to create kubeClient: %v", err)
		return false, nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("Unable to create dynamicClient: %v", err)
		return false, nil, err
	}

	clusterController := cluster.NewController(ctx.Opts.ClusterName, clientSet, dynamicClient, ctx.ClusterLinkClient)
	if err := mgr.Add(clusterController); err != nil {
		klog.Errorf("Failed to setup clustercontroller: %v", err)
		return false, nil, err
	}
	return true, nil, nil
}

func startNodeController(ctx ctrlcontext.Context) (bool, ctrlcontext.CleanFunc, error) {
	mgr := ctx.Mgr

	nodeController := &node.Reconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		ClusterName:       ctx.Opts.ClusterName,
		ClusterLinkClient: ctx.ClusterLinkClient,
	}
	if err := nodeController.SetupWithManager(mgr, ctx.Ctx.Done()); err != nil {
		return false, nil, err
	}
	return true, nodeController.CleanResource, nil
}

func startCalicoPoolController(ctx ctrlcontext.Context) (bool, ctrlcontext.CleanFunc, error) {
	mgr := ctx.Mgr
	restConfig := mgr.GetConfig()

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("Unable to create kubeClient: %v", err)
		return false, nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("Unable to create dynamicClient: %v", err)
		return false, nil, err
	}
	clusterLinkClient, err := versioned.NewForConfig(ctx.Opts.ControlPanelConfig)
	if err != nil {
		klog.Errorf("Unable to create clusterLinkClient: %v", err)
		return false, nil, err
	}
	controller := calicoippool.NewController(ctx.Opts.ClusterName, kubeClient, dynamicClient, clusterLinkClient)
	if err := mgr.Add(controller); err != nil {
		klog.Errorf("Failed to setup calicoippool controller: %v", err)
		return false, nil, err
	}
	return true, controller.CleanResource, nil
}

func startNodeCIDRController(ctx ctrlcontext.Context) (bool, ctrlcontext.CleanFunc, error) {
	mgr := ctx.Mgr

	nodeCIDRCtl := nodecidr.NewNodeCIDRController(mgr.GetConfig(), ctx.Opts.ClusterName, ctx.ClusterLinkClient, ctx.Opts.RateLimiterOpts, ctx.Ctx)
	if err := mgr.Add(nodeCIDRCtl); err != nil {
		klog.Fatalf("Failed to setup node CIDR Controller: %v", err)
		return true, nil, nil
	}
	return true, nil, nil
}
