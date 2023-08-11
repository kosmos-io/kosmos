package app

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kosmos.io/clusterlink/cmd/controller-manager/app/options"
	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	ctrlcontext "github.com/kosmos.io/clusterlink/pkg/controllers/context"
	"github.com/kosmos.io/clusterlink/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/clusterlink/pkg/generated/informers/externalversions"
	"github.com/kosmos.io/clusterlink/pkg/generated/listers/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/utils"
	"github.com/kosmos.io/clusterlink/pkg/utils/keys"
)

type Controller struct {
	processor         utils.AsyncWorker
	clusterLinkClient versioned.Interface
	clusterLister     v1alpha1.ClusterLister
	mgr               ctrl.Manager
	opts              *options.ControllerManagerOptions
	ctx               context.Context
	cleanFuncs        []ctrlcontext.CleanFunc
	cancelFunc        context.CancelFunc
	start             bool
}

func NewController(clusterLinkClient *versioned.Clientset, mgr ctrl.Manager, opts *options.ControllerManagerOptions) *Controller {
	return &Controller{
		clusterLinkClient: clusterLinkClient,
		mgr:               mgr,
		opts:              opts,
	}
}

func (c *Controller) Start(ctx context.Context) error {
	stopCh := ctx.Done()
	c.ctx = ctx
	opt := utils.Options{
		Name: "cluster Controller",
		KeyFunc: func(obj interface{}) (utils.QueueKey, error) {
			return keys.ClusterWideKeyFunc(obj)
		},
		ReconcileFunc:      c.Reconcile,
		RateLimiterOptions: c.opts.RateLimiterOpts,
	}
	c.processor = utils.NewAsyncWorker(opt)

	clusterInformerFactory := externalversions.NewSharedInformerFactory(c.clusterLinkClient, 0)
	clusterInformer := clusterInformerFactory.Clusterlink().V1alpha1().Clusters().Informer()
	clusterInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.OnAdd,
			UpdateFunc: c.OnUpdate,
			DeleteFunc: c.OnDelete,
		},
		FilterFunc: func(obj interface{}) bool {
			cluster, ok := obj.(*clusterlinkv1alpha1.Cluster)
			if !ok {
				return false
			}
			return cluster.Name == c.opts.ClusterName
		},
	})
	c.clusterLister = clusterInformerFactory.Clusterlink().V1alpha1().Clusters().Lister()

	c.setupControllers()

	c.processor.Run(1, stopCh)
	clusterInformerFactory.Start(stopCh)
	clusterInformerFactory.WaitForCacheSync(stopCh)
	<-stopCh
	klog.Infof("Stop controller as process done.")
	return nil
}

// OnAdd handles object add event and push the object to queue.
func (c *Controller) OnAdd(obj interface{}) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return
	}
	c.processor.Enqueue(runtimeObj)
}

// OnUpdate handles object update event and push the object to queue.
func (c *Controller) OnUpdate(oldObj, newObj interface{}) {
	runtimeObj, ok := newObj.(runtime.Object)
	if !ok {
		return
	}
	c.processor.Enqueue(runtimeObj)
}

// OnDelete handles object delete event and push the object to queue.
func (c *Controller) OnDelete(obj interface{}) {
	c.OnAdd(obj)
}

func (c *Controller) Reconcile(key utils.QueueKey) error {
	cluster, err := c.clusterLister.Get(c.opts.ClusterName)
	if err != nil {
		return err
	}
	if !cluster.DeletionTimestamp.IsZero() {
		c.cancelFunc()
		klog.Info("stop controllers")
		var errs []error
		klog.Info("clean resources")
		for _, cf := range c.cleanFuncs {
			err = cf()
			if err != nil {
				klog.Errorf("failed clean resource: %v", err)
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			return errors.NewAggregate(errs)
		}
		err := c.removeFinalizer(cluster)
		if err != nil {
			return err
		}
		return nil
	}

	//start controller
	if !c.start {
		c.startControllers()
		c.start = true
		err := c.ensureFinalizer(cluster)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) ensureFinalizer(cluster *clusterlinkv1alpha1.Cluster) error {
	if controllerutil.ContainsFinalizer(cluster, utils.ClusterStartControllerFinalizer) {
		return nil
	}

	controllerutil.AddFinalizer(cluster, utils.ClusterStartControllerFinalizer)
	_, err := c.clusterLinkClient.ClusterlinkV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("cluster %s failed add finalizer: %v", cluster.Name, err)
		return err
	}

	return nil
}

func (c *Controller) removeFinalizer(cluster *clusterlinkv1alpha1.Cluster) error {
	if !controllerutil.ContainsFinalizer(cluster, utils.ClusterStartControllerFinalizer) {
		return nil
	}
	controllerutil.RemoveFinalizer(cluster, utils.ClusterStartControllerFinalizer)
	_, err := c.clusterLinkClient.ClusterlinkV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("cluster %s failed remove finalizer: %v", cluster.Name, err)
		return err
	}
	return nil
}
func (c *Controller) setupControllers() {
	subCtx, cancelFunc := context.WithCancel(c.ctx)
	cleanFuns := setupControllers(c.mgr, c.opts, subCtx)
	c.cancelFunc = cancelFunc
	c.cleanFuncs = cleanFuns
}

func (c *Controller) startControllers() {

	//mgr的start会block
	go func() {
		if err := c.mgr.Start(c.ctx); err != nil {
			klog.Errorf("controller manager exits unexpectedly: %v", err)
			panic(err)
		}
	}()
}
