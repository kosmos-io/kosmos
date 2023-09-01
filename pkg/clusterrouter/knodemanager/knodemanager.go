package knodemanager

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	klogv2 "k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kosmos.io/kosmos/cmd/knode-manager/app/config"
	knodev1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	knode2 "github.com/kosmos.io/kosmos/pkg/clusterrouter/knodemanager/knode"
	crdclientset "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	knLister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
)

const KodeControllerFinalizer = "kosmos.io/knode-controller"
const defaultRetryNum = 5

type KnodeManager struct {
	runLock sync.Mutex
	stopCh  <-chan struct{}

	knclient        crdclientset.Interface
	informerFactory externalversions.SharedInformerFactory

	queue      workqueue.RateLimitingInterface
	knLister   knLister.KnodeLister
	knInformer cache.SharedIndexInformer

	knlock      sync.RWMutex
	knodes      map[string]*knode2.Knode
	knWaitGroup wait.Group

	opts *config.Opts
}

func NewManager(c *config.Config) *KnodeManager {
	factory := externalversions.NewSharedInformerFactory(c.CRDClient, 0)
	knInformer := factory.Kosmos().V1alpha1().Knodes()

	manager := &KnodeManager{
		knclient:        c.CRDClient,
		informerFactory: factory,
		knLister:        knInformer.Lister(),
		knInformer:      knInformer.Informer(),

		queue: workqueue.NewRateLimitingQueue(
			NewItemExponentialFailureAndJitterSlowRateLimter(2*time.Second, 15*time.Second, 1*time.Minute, 1.0, defaultRetryNum),
		),
		knodes: make(map[string]*knode2.Knode),
		opts:   c.Opts,
	}

	_, _ = knInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    manager.addKnode,
			UpdateFunc: manager.updateKnode,
			DeleteFunc: manager.deleteKnode,
		},
	)

	return manager
}

func (km *KnodeManager) addKnode(obj interface{}) {
	km.enqueue(obj)
}

func (km *KnodeManager) updateKnode(older, newer interface{}) {
	oldObj := older.(*knodev1alpha1.Knode)
	newObj := newer.(*knodev1alpha1.Knode)
	if newObj.DeletionTimestamp.IsZero() && equality.Semantic.DeepEqual(oldObj.Spec, newObj.Spec) {
		return
	}

	km.enqueue(newer)
}

func (km *KnodeManager) deleteKnode(obj interface{}) {
	km.enqueue(obj)
}

func (km *KnodeManager) enqueue(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}

	km.queue.Add(key)
}

func (km *KnodeManager) processNextCluster() (continued bool) {
	key, shutdown := km.queue.Get()
	if shutdown {
		return false
	}
	defer km.queue.Done(key)
	continued = true

	_, name, err := cache.SplitMetaNamespaceKey(key.(string))
	if err != nil {
		klogv2.Error(err)
		return
	}

	klogv2.InfoS("reconcile cluster", "virtualNode", name)
	knode, err := km.knLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klogv2.InfoS("cluster has been deleted", "virtual node", name)
			return
		}

		klogv2.ErrorS(err, "Failed to get cluster from cache", "cluvirtual node", name)
		return
	}

	knode = knode.DeepCopy()
	if result := km.reconcileKnode(knode); result.Requeue() {
		if num := km.queue.NumRequeues(key); num < result.MaxRetryCount() {
			klogv2.V(3).InfoS("requeue cluster", "cluster", name, "num requeues", num+1)
			km.queue.AddRateLimited(key)
			return
		}
		klogv2.V(2).Infof("Dropping cluster %q out of the queue: %v", key, err)
	}
	km.queue.Forget(key)
	return
}

func (km *KnodeManager) worker() {
	for km.processNextCluster() {
		select {
		case <-km.stopCh:
			return
		default:
		}
	}
}

func (km *KnodeManager) Run(workers int, stopCh <-chan struct{}) {
	km.runLock.Lock()
	defer km.runLock.Unlock()
	if km.stopCh != nil {
		klogv2.Fatal("virtualnode manager is already running...")
	}
	klogv2.Info("Start Informer Factory")

	// informerFactory should not be controlled by stopCh
	stopInformer := make(chan struct{})
	km.informerFactory.Start(stopInformer)
	if !cache.WaitForCacheSync(stopCh, km.knInformer.HasSynced) {
		klogv2.Fatal("virtualnode manager: wait for informer factory failed")
	}

	km.stopCh = stopCh

	klogv2.InfoS("Start Manager Cluster Worker", "workers", workers)
	var waitGroup sync.WaitGroup
	for i := 0; i < workers; i++ {
		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()
			wait.Until(km.worker, time.Second, km.stopCh)
		}()
	}

	<-km.stopCh
	klogv2.Info("receive stop signal, stop...")

	km.queue.ShutDown()
	waitGroup.Wait()

	klogv2.Info("wait for cluster synchros stop...")
	km.knWaitGroup.Wait()
	klogv2.Info("cluster synchro manager stopped.")
}

// if err returned is not nil, cluster will be requeued
func (km *KnodeManager) reconcileKnode(knode *knodev1alpha1.Knode) Result {
	if !knode.DeletionTimestamp.IsZero() {
		klogv2.InfoS("remove knode", "knode", knode.Name)
		if err := km.removeKnode(knode.Name); err != nil {
			klogv2.ErrorS(err, "Failed to remove knode", knode.Name)
			return RequeueResult(defaultRetryNum)
		}

		if !controllerutil.ContainsFinalizer(knode, KodeControllerFinalizer) {
			return NoRequeueResult
		}

		// remove finalizer
		controllerutil.RemoveFinalizer(knode, KodeControllerFinalizer)
		if _, err := km.knclient.KosmosV1alpha1().Knodes().Update(context.TODO(), knode, metav1.UpdateOptions{}); err != nil {
			klogv2.ErrorS(err, "Failed to remove finalizer", "knode", knode.Name)
			return RequeueResult(defaultRetryNum)
		}
		return NoRequeueResult
	}

	// ensure finalizer
	if !controllerutil.ContainsFinalizer(knode, KodeControllerFinalizer) {
		controllerutil.AddFinalizer(knode, KodeControllerFinalizer)

		if _, err := km.knclient.KosmosV1alpha1().Knodes().Update(context.TODO(), knode, metav1.UpdateOptions{}); err != nil {
			klogv2.ErrorS(err, "Failed to add finalizer", "virtual node", knode.Name)
			return RequeueResult(defaultRetryNum)
		}
	}

	km.knlock.RLock()
	kn := km.knodes[knode.Name]
	km.knlock.RUnlock()

	if kn == nil {
		opts := *km.opts
		opts.Plugin = knode.Spec.Type
		opts.NodeName = knode.Spec.NodeName
		opts.DisableTaint = knode.Spec.DisableTaint

		ctx := context.TODO()

		var err error
		kn, err = knode2.NewKnode(ctx, nil, &opts)
		if err != nil {
			klogv2.ErrorS(err, "Failed to new knode", "knode", knode.Name)
			return NoRequeueResult
		}
		go func() {
			kn.Run(ctx, &opts)
		}()

		km.knlock.RLock()
		km.knodes[knode.Name] = kn
		km.knlock.RUnlock()
	}

	return NoRequeueResult
}

func (km *KnodeManager) removeKnode(name string) error {
	km.knlock.Lock()
	knode := km.knodes[name]
	delete(km.knodes, name)
	km.knlock.Unlock()

	if knode != nil {
		// not update removed cluster status,
		// and ensure that no more data is being synchronized to the resource storage
		//knode.Shutdown()
		klogv2.Info("just for golint right")
	}

	// clean cluster from storage
	return nil
}

func NewItemExponentialFailureAndJitterSlowRateLimter(fastBaseDelay, fastMaxDelay, slowBaseDeploy time.Duration, slowMaxFactor float64, maxFastAttempts int) workqueue.RateLimiter {
	if slowMaxFactor <= 0.0 {
		slowMaxFactor = 1.0
	}
	return &ItemExponentialFailureAndJitterSlowRateLimter{
		failures:        map[interface{}]int{},
		maxFastAttempts: maxFastAttempts,
		fastBaseDelay:   fastBaseDelay,
		fastMaxDelay:    fastMaxDelay,
		slowBaseDelay:   slowBaseDeploy,
		slowMaxFactor:   slowMaxFactor,
	}
}

type ItemExponentialFailureAndJitterSlowRateLimter struct {
	failuresLock sync.Mutex
	failures     map[interface{}]int

	maxFastAttempts int

	fastBaseDelay time.Duration
	fastMaxDelay  time.Duration

	slowBaseDelay time.Duration
	slowMaxFactor float64
}

func (r *ItemExponentialFailureAndJitterSlowRateLimter) When(item interface{}) time.Duration {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()

	fastExp, num := r.failures[item], r.failures[item]+1
	r.failures[item] = num
	if num > r.maxFastAttempts {
		//nolint:gosec
		return r.slowBaseDelay + time.Duration(rand.Float64()*r.slowMaxFactor*float64(r.slowBaseDelay))
	}

	// The backoff is capped such that 'calculated' value never overflows.
	backoff := float64(r.fastBaseDelay.Nanoseconds()) * math.Pow(2, float64(fastExp))
	if backoff > math.MaxInt64 {
		return r.fastMaxDelay
	}

	calculated := time.Duration(backoff)
	if calculated > r.fastMaxDelay {
		return r.fastMaxDelay
	}
	return calculated
}

func (r *ItemExponentialFailureAndJitterSlowRateLimter) NumRequeues(item interface{}) int {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()

	return r.failures[item]
}

func (r *ItemExponentialFailureAndJitterSlowRateLimter) Forget(item interface{}) {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()

	delete(r.failures, item)
}
