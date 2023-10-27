package clusterManager

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/options"
	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/mcs"
	podcontrollers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pv"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pvc"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	ControllerName = "cluster-controller"
	RequeueTime    = 10 * time.Second

	ControllerFinalizerName    = "kosmos.io/cluster-manager" // TODO merge to constants
	RootClusterAnnotationKey   = "kosmos.io/cluster-role"
	RootClusterAnnotationValue = "root"

	DefaultLeafKubeQPS   = 40.0
	DefaultLeafKubeBurst = 60
)

type ClusterController struct {
	Root        client.Client
	RootDynamic dynamic.Interface
	RootClient  kubernetes.Interface

	EventRecorder record.EventRecorder
	Logger        logr.Logger
	Options       *options.Options

	ControllerManagers     map[string]*manager.Manager
	ManagerCancelFuncs     map[string]*context.CancelFunc
	ControllerManagersLock sync.Mutex

	mgr                 *manager.Manager
	RootResourceManager *utils.ResourceManager
}

func isRootCluster(cluster *clusterlinkv1alpha1.Cluster) bool {
	annotations := cluster.GetAnnotations()
	if val, ok := annotations[RootClusterAnnotationKey]; ok {
		return val == RootClusterAnnotationValue
	}
	return false
}

var predicatesFunc = predicate.Funcs{
	CreateFunc: func(createEvent event.CreateEvent) bool {
		obj := createEvent.Object.(*clusterlinkv1alpha1.Cluster)
		return !isRootCluster(obj)
	},
	UpdateFunc: func(updateEvent event.UpdateEvent) bool {
		obj := updateEvent.ObjectNew.(*clusterlinkv1alpha1.Cluster)
		old := updateEvent.ObjectOld.(*clusterlinkv1alpha1.Cluster)

		if isRootCluster(obj) {
			return false
		}

		// For now, only kubeconfig & DeletionTimestamp changes are concerned
		if !bytes.Equal(old.Spec.Kubeconfig, obj.Spec.Kubeconfig) {
			return true
		}

		if old.DeletionTimestamp != obj.DeletionTimestamp {
			return true
		}

		return false
	},
	DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
		obj := deleteEvent.Object.(*clusterlinkv1alpha1.Cluster)
		return !isRootCluster(obj)
	},
	GenericFunc: func(genericEvent event.GenericEvent) bool {
		return false
	},
}

func (c *ClusterController) SetupWithManager(mgr manager.Manager) error {
	c.ManagerCancelFuncs = make(map[string]*context.CancelFunc)
	c.ControllerManagers = make(map[string]*manager.Manager)
	c.Logger = mgr.GetLogger()

	// TODO this may not be a good idea
	c.mgr = &mgr
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{}).
		For(&clusterlinkv1alpha1.Cluster{}, builder.WithPredicates(predicatesFunc)).
		Complete(c)
}

func (c *ClusterController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", ControllerName, request.Name)

	cluster := &clusterlinkv1alpha1.Cluster{}
	if err := c.Root.Get(ctx, request.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("Cluster %s has been deleted", request.Name)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{RequeueAfter: RequeueTime}, err
	}

	config, err := utils.NewConfigFromBytes(cluster.Spec.Kubeconfig, func(config *rest.Config) {
		config.QPS = DefaultLeafKubeQPS
		config.Burst = DefaultLeafKubeBurst
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not build kubeconfig for cluster %s: %v", cluster.Name, err)
	}

	leafClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not build clientset for cluster %s: %v", cluster.Name, err)
	}

	leafDynamic, err := dynamic.NewForConfig(config)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not build dynamic client for cluster %s: %v", cluster.Name, err)
	}

	kosmosClient, err := kosmosversioned.NewForConfig(config)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not build kosmos clientset for cluster %s: %v", cluster.Name, err)
	}

	// ensure finalizer
	if cluster.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(cluster, ControllerFinalizerName) {
			controllerutil.AddFinalizer(cluster, ControllerFinalizerName)
			if err := c.Root.Update(ctx, cluster); err != nil {
				return controllerruntime.Result{}, err
			}
		}
	}

	// cluster deleted || cluster added || kubeconfig changed
	c.clearClusterControllers(cluster)

	if !cluster.DeletionTimestamp.IsZero() {
		if err := c.deleteNode(ctx, cluster); err != nil {
			return reconcile.Result{
				Requeue: true,
			}, err
		}
		if controllerutil.ContainsFinalizer(cluster, ControllerFinalizerName) {
			controllerutil.RemoveFinalizer(cluster, ControllerFinalizerName)
			if err := c.Root.Update(ctx, cluster); err != nil {
				return controllerruntime.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	node, err := c.createNode(ctx, cluster, leafClient)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("create node with err %v, cluster %s", err, cluster.Name)
	}
	// TODO @wyz
	node.ResourceVersion = ""

	// build mgr for cluster
	// TODO bug, the v4 log is lost
	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Logger:                 c.Logger.WithName("leaf-controller-manager"),
		Scheme:                 scheme.NewSchema(),
		LeaderElection:         false,
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("new manager with err %v, cluster %s", err, cluster.Name)
	}

	subContext, cancel := context.WithCancel(ctx)

	c.ControllerManagersLock.Lock()
	c.ControllerManagers[cluster.Name] = &mgr
	c.ManagerCancelFuncs[cluster.Name] = &cancel
	c.ControllerManagersLock.Unlock()

	if err = c.setupControllers(&mgr, cluster, node, leafDynamic, leafClient, kosmosClient); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to setup cluster %s controllers: %v", cluster.Name, err)
	}

	go func() {
		if err := mgr.Start(subContext); err != nil {
			klog.Errorf("failed to start cluster %s controller manager: %v", cluster.Name, err)
		}
	}()

	klog.V(4).Infof("============ %s has been reconciled =============", request.Name)

	return reconcile.Result{}, nil
}

func (c *ClusterController) clearClusterControllers(cluster *clusterlinkv1alpha1.Cluster) {
	c.ControllerManagersLock.Lock()
	defer c.ControllerManagersLock.Unlock()

	if f, ok := c.ManagerCancelFuncs[cluster.Name]; ok {
		cancel := *f
		cancel()
	}
	delete(c.ManagerCancelFuncs, cluster.Name)
	delete(c.ControllerManagers, cluster.Name)
}

func (c *ClusterController) setupControllers(m *manager.Manager, cluster *clusterlinkv1alpha1.Cluster, node *corev1.Node, clientDynamic *dynamic.DynamicClient, leafClient kubernetes.Interface, kosmosClient kosmosversioned.Interface) error {
	mgr := *m
	nodeResourcesController := controllers.NodeResourcesController{
		Leaf:          mgr.GetClient(),
		Root:          c.Root,
		RootClientset: c.RootClient,
		Node:          node,
	}
	if err := nodeResourcesController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", controllers.NodeResourcesControllerName, err)
	}

	nodeLeaseController := controllers.NewNodeLeaseController(leafClient, c.Root, node, c.RootClient)
	if err := mgr.Add(nodeLeaseController); err != nil {
		return fmt.Errorf("error starting %s: %v", controllers.NodeLeaseControllerName, err)
	}

	serviceImportController := &mcs.ServiceImportController{
		LeafClient:          mgr.GetClient(),
		RootClient:          c.Root,
		RootKosmosClient:    kosmosClient,
		EventRecorder:       mgr.GetEventRecorderFor(mcs.LeafServiceImportControllerName),
		Logger:              mgr.GetLogger(),
		LeafNodeName:        cluster.Name,
		RootResourceManager: c.RootResourceManager,
	}

	if err := serviceImportController.AddController(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", mcs.LeafServiceImportControllerName, err)
	}

	// TODO Consider moving up to the same level as cluster-controller, add controllers after mgr is started may cause problems ？
	RootPodReconciler := podcontrollers.RootPodReconciler{
		LeafClient:           mgr.GetClient(),
		RootClient:           c.Root,
		NodeName:             cluster.Name,
		Namespace:            cluster.Spec.Namespace,
		IgnoreLabels:         strings.Split("", ","),
		EnableServiceAccount: true,
		DynamicRootClient:    c.RootDynamic,
		DynamicLeafClient:    clientDynamic,
	}
	if err := RootPodReconciler.SetupWithManager(*c.mgr); err != nil {
		return fmt.Errorf("error starting RootPodReconciler %s: %v", podcontrollers.RootPodControllerName, err)
	}

	podUpstreamController := podcontrollers.LeafPodReconciler{
		RootClient: c.Root,
		Namespace:  cluster.Spec.Namespace,
	}

	if err := podUpstreamController.SetupWithManager(*c.mgr); err != nil {
		return fmt.Errorf("error starting podUpstreamReconciler %s: %v", podcontrollers.LeafPodControllerName, err)
	}

	err := c.setupStorageControllers(m, node, leafClient)
	if err != nil {
		return err
	}

	for i, gvr := range podcontrollers.SYNC_GVRS {
		demoController := podcontrollers.SyncResourcesReconciler{
			GroupVersionResource: gvr,
			Object:               podcontrollers.SYNC_OBJS[i],
			DynamicRootClient:    c.RootDynamic,
			DynamicLeafClient:    clientDynamic,
			ControllerName:       "async-controller-" + gvr.Resource,
		}
		if err := demoController.SetupWithManager(mgr, gvr); err != nil {
			klog.Errorf("Unable to create cluster node controller: %v", err)
			return err
		}
	}

	return nil
}

func (c *ClusterController) setupStorageControllers(m *manager.Manager, node *corev1.Node, leafClient kubernetes.Interface) error {
	mgr := *m

	rootPVCController := pvc.RootPVCController{
		LeafClient:    mgr.GetClient(),
		RootClient:    c.Root,
		LeafClientSet: leafClient,
	}
	if err := rootPVCController.SetupWithManager(*c.mgr); err != nil {
		return fmt.Errorf("error starting root pvc controller %v", err)
	}

	rootPVController := pv.RootPVController{
		LeafClient:    mgr.GetClient(),
		RootClient:    c.Root,
		LeafClientSet: leafClient,
	}
	if err := rootPVController.SetupWithManager(*c.mgr); err != nil {
		return fmt.Errorf("error starting root pv controller %v", err)
	}

	leafPVCController := pvc.LeafPVCController{
		LeafClient:    mgr.GetClient(),
		RootClient:    c.Root,
		RootClientSet: c.RootClient,
		NodeName:      node.Name,
	}
	if err := leafPVCController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting leaf pvc controller %v", err)
	}

	leafPVontroller := pv.LeafPVController{
		LeafClient:    mgr.GetClient(),
		RootClient:    c.Root,
		RootClientSet: c.RootClient,
		NodeName:      node.Name,
	}
	if err := leafPVontroller.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting leaf pv controller %v", err)
	}

	return nil
}

func (c *ClusterController) createNode(ctx context.Context, cluster *clusterlinkv1alpha1.Cluster, leafClient kubernetes.Interface) (*corev1.Node, error) {
	serverVersion, err := leafClient.Discovery().ServerVersion()
	if err != nil {
		klog.Errorf("create node failed, can not connect to leaf %s", cluster.Name)
		return nil, err
	}

	node, err := c.RootClient.CoreV1().Nodes().Get(ctx, cluster.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("create node failed, can not get node %s", cluster.Name)
		return nil, err
	}

	if err != nil && errors.IsNotFound(err) {
		node = utils.BuildNodeTemplate(cluster)
		node.Status.NodeInfo.KubeletVersion = serverVersion.GitVersion
		node.Status.DaemonEndpoints = corev1.NodeDaemonEndpoints{
			KubeletEndpoint: corev1.DaemonEndpoint{
				Port: c.Options.ListenPort,
			},
		}
		node, err = c.RootClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			klog.Errorf("create node %s failed, err: %v", cluster.Name, err)
			return nil, err
		}
	}
	return node, nil
}

func (c *ClusterController) deleteNode(ctx context.Context, cluster *clusterlinkv1alpha1.Cluster) error {
	err := c.RootClient.CoreV1().Nodes().Delete(ctx, cluster.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}
