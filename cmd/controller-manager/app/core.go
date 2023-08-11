package app

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"cnp.io/clusterlink/pkg/controllers/calicoippool"
	"cnp.io/clusterlink/pkg/controllers/cluster"
	ctrlcontext "cnp.io/clusterlink/pkg/controllers/context"
	"cnp.io/clusterlink/pkg/controllers/node"
	"cnp.io/clusterlink/pkg/generated/clientset/versioned"
)

func startClusterController(ctx ctrlcontext.Context) (enabled bool, err error) {
	mgr := ctx.Mgr
	restConfig := mgr.GetConfig()

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("Unable to create kubeClient: %v", err)
		return false, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("Unable to create dynamicClient: %v", err)
		return false, err
	}

	clusterLinkClient, err := versioned.NewForConfig(ctx.Opts.ControlPanelConfig)
	clusterController := cluster.NewController(ctx.Opts.ClusterName, clientSet, dynamicClient, clusterLinkClient)
	if err := mgr.Add(clusterController); err != nil {
		klog.Errorf("Failed to setup clustercontroller: %v", err)
		return false, err
	}
	return true, nil
}

func startNodeController(ctx ctrlcontext.Context) (enabled bool, err error) {
	mgr := ctx.Mgr
	clusterLinkClient, err := versioned.NewForConfig(ctx.Opts.ControlPanelConfig)
	if err != nil {
		klog.Errorf("Unable to create clusterLink client: %v", err)
		return false, err
	}

	nodeController := &node.Reconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		ClusterName:       ctx.Opts.ClusterName,
		ClusterLinkClient: clusterLinkClient,
		//IPPoolManager: ipPoolMgr,
	}
	if err := nodeController.SetupWithManager(mgr, ctx.StopChan); err != nil {
		return false, err
	}
	return true, nil
}

func startCalicoPoolController(ctx ctrlcontext.Context) (enabled bool, err error) {
	mgr := ctx.Mgr
	restConfig := mgr.GetConfig()

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("Unable to create kubeClient: %v", err)
		return false, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("Unable to create dynamicClient: %v", err)
		return false, err
	}
	clusterLinkClient, err := versioned.NewForConfig(ctx.Opts.ControlPanelConfig)
	if err != nil {
		klog.Errorf("Unable to create clusterLinkClient: %v", err)
		return false, err
	}
	controller := calicoippool.NewController(ctx.Opts.ClusterName, kubeClient, dynamicClient, clusterLinkClient)
	if err := mgr.Add(controller); err != nil {
		klog.Errorf("Failed to setup calicoippool controller: %v", err)
		return false, err
	}
	return true, nil
}
