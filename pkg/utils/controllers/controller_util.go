package controllers

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	// maxRetries is the number of times a runtime object will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the times
	// an object is going to be requeued:
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15
)

type Reconcile func(string) error

type Worker interface {
	// Enqueue enqueue a event object key into queue without block
	Enqueue(obj runtime.Object)

	// EnqueueRateLimited enqueue an event object key into queue after the rate limiter says it's ok
	EnqueueRateLimited(obj runtime.Object)

	// EnqueueAfter enqueue an event object key into queue after the indicated duration has passed
	EnqueueAfter(obj runtime.Object, after time.Duration)

	// Forget remove an event object key out of queue
	Forget(obj runtime.Object)

	// GetFirst for test
	GetFirst() (string, error)

	// Run will not return until stopChan is closed.
	Run(concurrency int, stopChan <-chan struct{})

	// SplitKey returns the namespace and name that
	// MetaNamespaceKeyFunc encoded into key.
	SplitKey(key string) (namespace, name string, err error)
}

type worker struct {
	// runtime Objects keys that need to be synced.
	queue workqueue.RateLimitingInterface

	// reconcile function to handle keys
	reconcile Reconcile

	// keyFunc encoded an object into string
	keyFunc func(obj interface{}) (string, error)
}

// NewWorker returns a Concurrent informer worker which can process resource event.
func NewWorker(reconcile Reconcile, name string) Worker {
	return &worker{
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		reconcile: reconcile,
		keyFunc:   cache.DeletionHandlingMetaNamespaceKeyFunc,
	}
}

func (c *worker) Enqueue(obj runtime.Object) {
	key, err := c.keyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", obj, err))
		return
	}

	c.queue.Add(key)
}

func (c *worker) EnqueueRateLimited(obj runtime.Object) {
	key, err := c.keyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", obj, err))
		return
	}

	c.queue.AddRateLimited(key)
}

func (c *worker) EnqueueAfter(obj runtime.Object, after time.Duration) {
	key, err := c.keyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", obj, err))
		return
	}

	c.queue.AddAfter(key, after)
}

func (c *worker) GetFirst() (string, error) {
	item, _ := c.queue.Get()
	return item.(string), nil
}

func (c *worker) Forget(obj runtime.Object) {
	key, err := c.keyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", obj, err))
		return
	}

	c.queue.Forget(key)
}

func (c *worker) Run(workerNumber int, stopChan <-chan struct{}) {
	defer c.queue.ShutDown()

	for i := 0; i < workerNumber; i++ {
		go wait.Until(c.worker, time.Second, stopChan)
	}

	<-stopChan
}

func (c *worker) SplitKey(key string) (namespace, name string, err error) {
	return cache.SplitMetaNamespaceKey(key)
}

// worker runs a worker thread that just dequeues items, processes them, and
// marks them done. You may run as many of these in parallel as you wish; the
// queue guarantees that they will not end up processing the same runtime object
// at the same time
func (c *worker) worker() {
	for c.processNextItem() {
	}
}

func (c *worker) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(key)

	err := c.reconcile(key.(string))
	c.handleErr(err, key)
	return true
}

func (c *worker) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < maxRetries {
		c.queue.AddRateLimited(key)
		return
	}

	klog.V(2).Infof("Dropping resource %q out of the queue: %v", key, err)
	c.queue.Forget(key)
}
