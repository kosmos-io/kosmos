package runtime

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const (
	K8S int = iota
	OPENAPI
)

type Func func(key NamespacedName) (Result, error)

type Reconciler interface {
	Reconcile(ctx context.Context, key NamespacedName) (Result, error)
}

type Controller struct {
	// indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller
	kind     int
	Do       Reconciler
	Workers  int
}

func NewOpenApiWorkerQueue(queue workqueue.RateLimitingInterface, r Reconciler) Controller {
	return Controller{
		queue:   queue,
		kind:    OPENAPI,
		Do:      r,
		Workers: 1,
	}
}

func NewK8sWorkerQueue(queue workqueue.RateLimitingInterface, informer cache.SharedIndexInformer, r Reconciler) Controller {
	return Controller{
		queue:    queue,
		kind:     K8S,
		informer: informer,
		Do:       r,
		Workers:  1,
	}
}

func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextItem(ctx) {
	}
}
func (c *Controller) processNextItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key.
	defer c.queue.Done(key)

	// Invoke the method containing the business logic
	result, err := c.Reconcile(ctx, key.(NamespacedName))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(result, err, key)
	return true
}

func (c *Controller) Run(ctx context.Context) {
	defer runtime.HandleCrash()

	stopCh := ctx.Done()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()
	klog.Info("Starting Pod controller")

	if c.kind == K8S {
		go c.informer.Run(stopCh)

		// Wait for all involved caches to be synced, before processing items from the queue is started
		if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
			runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
			return
		}
	}

	for i := 0; i < c.Workers; i++ {
		go wait.Until(func() { c.runWorker(ctx) }, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("Stopping Pod controller")
}

func (c *Controller) handleErr(result Result, err error, key interface{}) {
	if !result.Requeue && result.RequeueAfter == 0 {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	if err != nil {
		klog.Fatal(err, key)
	}

	if result.RequeueAfter != 0 {
		c.queue.AddAfter(key, result.RequeueAfter)
	} else {
		c.queue.AddRateLimited(key)
	}
}

func (c *Controller) Reconcile(ctx context.Context, key NamespacedName) (Result, error) {
	return c.Do.Reconcile(ctx, key)
}
