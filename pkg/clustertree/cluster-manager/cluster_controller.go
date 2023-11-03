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
	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/mcs"
	podcontrollers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pv"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pvc"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	ControllerName = "cluster-controller"
	RequeueTime    = 10 * time.Second

	ControllerFinalizerName = "kosmos.io/cluster-manager" // TODO merge to constants
)

type ClusterController struct {
	Root        client.Client
	RootDynamic dynamic.Interface
	RootClient  kubernetes.Interface

	EventRecorder record.EventRecorder
	Logger        logr.Logger
	Options       *options.Options

	ControllerManagers     map[string]manager.Manager
	ManagerCancelFuncs     map[string]*context.CancelFunc
	ControllerManagersLock sync.Mutex

	RootResourceManager *utils.ResourceManager

	GlobalLeafManager leafUtils.LeafResourceManager
}

var predicatesFunc = predicate.Funcs{
	CreateFunc: func(createEvent event.CreateEvent) bool {
		obj := createEvent.Object.(*kosmosv1alpha1.Cluster)
		return !leafUtils.IsRootCluster(obj)
	},
	UpdateFunc: func(updateEvent event.UpdateEvent) bool {
		obj := updateEvent.ObjectNew.(*kosmosv1alpha1.Cluster)
		old := updateEvent.ObjectOld.(*kosmosv1alpha1.Cluster)

		if leafUtils.IsRootCluster(obj) {
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
		obj := deleteEvent.Object.(*kosmosv1alpha1.Cluster)
		return !leafUtils.IsRootCluster(obj)
	},
	GenericFunc: func(genericEvent event.GenericEvent) bool {
		return false
	},
}

func (c *ClusterController) SetupWithManager(mgr manager.Manager) error {
	c.ManagerCancelFuncs = make(map[string]*context.CancelFunc)
	c.ControllerManagers = make(map[string]manager.Manager)
	c.Logger = mgr.GetLogger()
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{}).
		For(&kosmosv1alpha1.Cluster{}, builder.WithPredicates(predicatesFunc)).
		Complete(c)
}

func (c *ClusterController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", ControllerName, request.Name)

	cluster := &kosmosv1alpha1.Cluster{}
	if err := c.Root.Get(ctx, request.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("Cluster %s has been deleted", request.Name)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{RequeueAfter: RequeueTime}, err
	}

	config, err := utils.NewConfigFromBytes(cluster.Spec.Kubeconfig, func(config *rest.Config) {
		config.QPS = utils.DefaultLeafKubeQPS
		config.Burst = utils.DefaultLeafKubeBurst
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
	c.ControllerManagers[cluster.Name] = mgr
	c.ManagerCancelFuncs[cluster.Name] = &cancel
	c.ControllerManagersLock.Unlock()

	if err = c.setupControllers(mgr, cluster, node, leafDynamic, leafClient, kosmosClient); err != nil {
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

func (c *ClusterController) clearClusterControllers(cluster *kosmosv1alpha1.Cluster) {
	c.ControllerManagersLock.Lock()
	defer c.ControllerManagersLock.Unlock()

	if f, ok := c.ManagerCancelFuncs[cluster.Name]; ok {
		cancel := *f
		cancel()
	}
	delete(c.ManagerCancelFuncs, cluster.Name)
	delete(c.ControllerManagers, cluster.Name)

	c.GlobalLeafManager.RemoveLeafResource(cluster.Name)
}

func (c *ClusterController) setupControllers(mgr manager.Manager, cluster *kosmosv1alpha1.Cluster, node *corev1.Node, clientDynamic *dynamic.DynamicClient, leafClient kubernetes.Interface, kosmosClient kosmosversioned.Interface) error {
	nodeName := fmt.Sprintf("%s%s", utils.KosmosNodePrefix, cluster.Name)
	c.GlobalLeafManager.AddLeafResource(nodeName, &leafUtils.LeafResource{
		Client:               mgr.GetClient(),
		DynamicClient:        clientDynamic,
		Clientset:            leafClient,
		KosmosClient:         kosmosClient,
		NodeName:             nodeName,
		Namespace:            "",
		IgnoreLabels:         strings.Split("", ","),
		EnableServiceAccount: true,
	})

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

	if c.Options.MultiClusterService {
		serviceImportController := &mcs.ServiceImportController{
			LeafClient:          mgr.GetClient(),
			RootKosmosClient:    kosmosClient,
			EventRecorder:       mgr.GetEventRecorderFor(mcs.LeafServiceImportControllerName),
			Logger:              mgr.GetLogger(),
			LeafNodeName:        nodeName,
			RootResourceManager: c.RootResourceManager,
		}
		if err := serviceImportController.AddController(mgr); err != nil {
			return fmt.Errorf("error starting %s: %v", mcs.LeafServiceImportControllerName, err)
		}
	}

	leafPodController := podcontrollers.LeafPodReconciler{
		RootClient: c.Root,
		Namespace:  "",
	}

	if err := leafPodController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting podUpstreamReconciler %s: %v", podcontrollers.LeafPodControllerName, err)
	}

	err := c.setupStorageControllers(mgr, node, leafClient)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClusterController) setupStorageControllers(mgr manager.Manager, node *corev1.Node, leafClient kubernetes.Interface) error {
	leafPVCController := pvc.LeafPVCController{
		LeafClient:    mgr.GetClient(),
		RootClient:    c.Root,
		RootClientSet: c.RootClient,
		NodeName:      node.Name,
	}
	if err := leafPVCController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting leaf pvc controller %v", err)
	}

	leafPVController := pv.LeafPVController{
		LeafClient:    mgr.GetClient(),
		RootClient:    c.Root,
		RootClientSet: c.RootClient,
		NodeName:      node.Name,
	}
	if err := leafPVController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting leaf pv controller %v", err)
	}
	return nil
}

func (c *ClusterController) createNode(ctx context.Context, cluster *kosmosv1alpha1.Cluster, leafClient kubernetes.Interface) (*corev1.Node, error) {
	nodeName := fmt.Sprintf("%s%s", utils.KosmosNodePrefix, cluster.Name)
	serverVersion, err := leafClient.Discovery().ServerVersion()
	if err != nil {
		klog.Errorf("create node failed, can not connect to leaf %s", nodeName)
		return nil, err
	}

	node, err := c.RootClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("create node failed, can not get node %s", nodeName)
		return nil, err
	}

	if err != nil && errors.IsNotFound(err) {
		node = utils.BuildNodeTemplate(nodeName)
		node.Status.NodeInfo.KubeletVersion = serverVersion.GitVersion
		node.Status.DaemonEndpoints = corev1.NodeDaemonEndpoints{
			KubeletEndpoint: corev1.DaemonEndpoint{
				Port: c.Options.ListenPort,
			},
		}
		node, err = c.RootClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			klog.Errorf("create node %s failed, err: %v", nodeName, err)
			return nil, err
		}
	}
	return node, nil
}

func (c *ClusterController) deleteNode(ctx context.Context, cluster *kosmosv1alpha1.Cluster) error {
	err := c.RootClient.CoreV1().Nodes().Delete(ctx, cluster.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}
