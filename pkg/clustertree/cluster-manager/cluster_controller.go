package clusterManager

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
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

	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	networkmanager "github.com/kosmos.io/kosmos/pkg/clusterlink/network-manager"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers"
	"github.com/kosmos.io/kosmos/pkg/scheme"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	ControllerName = "cluster-controller"
	//RequeueTime    = 5 * time.Second

	ControllerFinalizerName      = "kosmos.io/cluster-manager"
	MasterClusterAnnotationKey   = "kosmos.io/cluster-role"
	MasterClusterAnnotationValue = "master"

	DefaultClusterKubeQPS   = 40.0
	DefalutClusterKubeBurst = 60
)

type ClusterController struct {
	Master        client.Client
	EventRecorder record.EventRecorder
	Logger        logr.Logger

	ConfigOptFunc func(config *rest.Config)

	MasterResourceManager *utils.ResourceManager

	// clusterName: Manager
	ControllerManagers     map[string]*manager.Manager
	ManagerCancelFuncs     map[string]*context.CancelFunc
	ControllerManagersLock sync.Mutex
}

func isMasterCluster(cluster *clusterlinkv1alpha1.Cluster) bool {
	annotations := cluster.GetAnnotations()
	if val, ok := annotations[MasterClusterAnnotationKey]; ok {
		return val == MasterClusterAnnotationValue
	}
	return false
}

var predicatesFunc = predicate.Funcs{
	CreateFunc: func(createEvent event.CreateEvent) bool {
		obj := createEvent.Object.(*clusterlinkv1alpha1.Cluster)
		return !isMasterCluster(obj)
	},
	UpdateFunc: func(updateEvent event.UpdateEvent) bool {
		obj := updateEvent.ObjectNew.(*clusterlinkv1alpha1.Cluster)
		old := updateEvent.ObjectOld.(*clusterlinkv1alpha1.Cluster)

		if isMasterCluster(obj) {
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
		return !isMasterCluster(obj)
	},
	GenericFunc: func(genericEvent event.GenericEvent) bool {
		return false
	},
}

func (c *ClusterController) SetupWithManager(mgr manager.Manager) error {
	c.ManagerCancelFuncs = make(map[string]*context.CancelFunc)
	c.ControllerManagers = make(map[string]*manager.Manager)
	c.Logger = mgr.GetLogger()

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{}).
		For(&clusterlinkv1alpha1.Cluster{}, builder.WithPredicates(predicatesFunc)).
		Complete(c)
}

func (c *ClusterController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", ControllerName, request.Name)
	defer func() {
		klog.V(4).Infof("============ %s has been reconciled =============", request.Name)
	}()

	cluster := &clusterlinkv1alpha1.Cluster{}
	if err := c.Master.Get(ctx, request.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("Cluster %s has been deleted", request.Name)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	// ensure finalizer
	if cluster.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(cluster, ControllerFinalizerName) {
			controllerutil.AddFinalizer(cluster, ControllerFinalizerName)
			if err := c.Master.Update(ctx, cluster); err != nil {
				return controllerruntime.Result{}, err
			}
		}
	}

	if !cluster.DeletionTimestamp.IsZero() {
		c.clearClusterControllers(cluster)

		if controllerutil.ContainsFinalizer(cluster, ControllerFinalizerName) {
			controllerutil.RemoveFinalizer(cluster, ControllerFinalizerName)
			if err := c.Master.Update(ctx, cluster); err != nil {
				return controllerruntime.Result{}, err
			}
		}
	}

	// cluster added or kubeconfig changed
	c.clearClusterControllers(cluster)

	// build mgr for cluster
	config, err := utils.NewConfigFromBytes(cluster.Spec.Kubeconfig, func(config *rest.Config) {
		config.QPS = DefaultClusterKubeQPS
		config.Burst = DefalutClusterKubeBurst
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not build clientset for cluster %s: %v", cluster.Name, err)
	}

	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Logger:                 c.Logger.WithName("cluster-controller-manager"),
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

	if err = c.setupControllers(&mgr, cluster.Name); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to setup cluster %s controllers: %v", cluster.Name, err)
	}

	go func() {
		if err := mgr.Start(subContext); err != nil {
			klog.Errorf("failed to start cluster %s controller manager: %v", cluster.Name, err)
		}
	}()

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

func (c *ClusterController) setupControllers(m *manager.Manager, clusterName string) error {
	mgr := *m

	nodeResourcesController := controllers.NodeResourcesController{
		Client: mgr.GetClient(),
		Master: c.Master,
	}
	if err := nodeResourcesController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", networkmanager.ControllerName, err)
	}

	//// mcs controller
	//clusterKubeClient, err := utils.NewClusterKubeClient(mgr.GetClient(), clusterName, c.ConfigOptFunc)
	//if err != nil {
	//	return err
	//}

	clusterKosmosClient, err := utils.NewClusterKosmosClient(mgr.GetClient(), clusterName, c.ConfigOptFunc)
	if err != nil {
		return err
	}

	serviceImportController := &controllers.ServiceImportController{
		Client:              mgr.GetClient(),
		Master:              c.Master,
		EventRecorder:       mgr.GetEventRecorderFor(controllers.MemberServiceImportControllerName),
		Logger:              mgr.GetLogger(),
		ClusterNodeName:     clusterName,
		ClusterKosmosClient: clusterKosmosClient,
	}

	if err := serviceImportController.AddController(mgr); err != nil {
		return fmt.Errorf("error starting %s: %v", controllers.MemberServiceImportControllerName, err)
	}

	return nil
}
