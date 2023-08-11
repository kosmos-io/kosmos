package nodecidr

import (
	"context"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/clusterlink/pkg/generated/informers/externalversions"
	clusterinformer "github.com/kosmos.io/clusterlink/pkg/generated/informers/externalversions/clusterlink/v1alpha1"
	clusterlister "github.com/kosmos.io/clusterlink/pkg/generated/listers/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/utils"
	"github.com/kosmos.io/clusterlink/pkg/utils/flags"
	"github.com/kosmos.io/clusterlink/pkg/utils/keys"
)

const (
	workNum     = 1
	requeueTime = 10 * time.Second
	calicoCNI   = "calico"
)

type controller struct {
	clusterName string
	config      *rest.Config

	clusterLinkClient versioned.Interface

	// RateLimiterOptions is the configuration for rate limiter which may significantly influence the performance of
	// the controller.
	RateLimiterOptions  flags.Options
	processor           utils.AsyncWorker
	clusterNodeInformer clusterinformer.ClusterNodeInformer
	clusterNodeLister   clusterlister.ClusterNodeLister

	cniAdapter cniAdapter
	sync.RWMutex
	ctx context.Context
}

func NewNodeCIDRController(config *rest.Config, clusterName string, clusterLinkClient versioned.Interface, RateLimiterOptions flags.Options, context context.Context) *controller {
	return &controller{
		clusterLinkClient:  clusterLinkClient,
		config:             config,
		RateLimiterOptions: RateLimiterOptions,
		ctx:                context,
		clusterName:        clusterName,
	}
}

func (c *controller) Start(ctx context.Context) error {
	klog.Infof("Starting node cidr controller.")

	opt := utils.Options{
		Name:               "node cidr controller",
		KeyFunc:            ClusterWideKeyFunc,
		ReconcileFunc:      c.Reconcile,
		RateLimiterOptions: c.RateLimiterOptions,
	}
	c.processor = utils.NewAsyncWorker(opt)

	clusterInformerFactory := externalversions.NewSharedInformerFactory(c.clusterLinkClient, 0)

	c.clusterNodeInformer = clusterInformerFactory.Clusterlink().V1alpha1().ClusterNodes()
	_, err := c.clusterNodeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.OnAdd,
			UpdateFunc: c.OnUpdate,
			DeleteFunc: c.OnDelete,
		},
		FilterFunc: func(obj interface{}) bool {
			node, ok := obj.(clusterlinkv1alpha1.ClusterNode)
			if !ok {
				return false
			}
			return node.Spec.ClusterName == c.clusterName
		},
	})
	if err != nil {
		return err
	}

	c.clusterNodeLister = clusterInformerFactory.Clusterlink().V1alpha1().ClusterNodes().Lister()
	cluster, err := c.clusterLinkClient.ClusterlinkV1alpha1().Clusters().Get(ctx, c.clusterName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("can not find local cluster %s, err: %v", c.clusterName, err)
		return err
	}

	stopCh := c.ctx.Done()
	clusterInformerFactory.Start(stopCh)
	clusterInformerFactory.WaitForCacheSync(stopCh)

	// third step: init CNI Adapter
	if cluster.Spec.CNI == calicoCNI {
		c.cniAdapter = NewCalicoAdapter(c.config, c.clusterNodeLister, c.processor)
	} else {
		c.cniAdapter = NewCommonAdapter(c.config, c.clusterNodeLister, c.processor)
	}
	err = c.cniAdapter.start(stopCh)
	if err != nil {
		return err
	}

	// last step : start processor and waiting for close chan
	c.processor.Run(workNum, stopCh)
	<-stopCh
	klog.Infof("Stop node cidr controller as process done.")
	return nil
}

func (c *controller) Reconcile(key utils.QueueKey) error {
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Error("invalid key")
		return fmt.Errorf("invalid key")
	}
	klog.Infof("Reconciling Cluster Node: %s", clusterWideKey)
	clusterNode, err := c.clusterNodeLister.Get(clusterWideKey.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Cluster Node %s has been removed.", clusterWideKey.NamespaceKey())
			return nil
		}
		return err
	}

	originNodeName := clusterNode.Spec.NodeName
	originClusterName := clusterNode.Spec.ClusterName

	if originClusterName == "" || originNodeName == "" {
		klog.Infof("Cluster Node %s has not been synced Ready.", clusterWideKey.NamespaceKey())
		c.processor.AddAfter(key, requeueTime)
		return nil
	}

	if !c.cniAdapter.synced() {
		c.processor.AddAfter(key, requeueTime)
		return nil
	}

	podCIDRs, err := c.cniAdapter.getCIDRByNodeName(originNodeName)
	if err != nil {
		return err
	}

	clusterNodeCopy := clusterNode.DeepCopy()
	clusterNodeCopy.Spec.PodCIDRs = podCIDRs

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = c.clusterLinkClient.ClusterlinkV1alpha1().ClusterNodes().Update(context.TODO(), clusterNodeCopy, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
}

// ClusterWideKeyFunc generates a ClusterWideKey for object.
func ClusterWideKeyFunc(obj interface{}) (utils.QueueKey, error) {
	return keys.ClusterWideKeyFunc(obj)
}

// OnAdd handles object add event and push the object to queue.
func (c *controller) OnAdd(obj interface{}) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return
	}
	c.processor.Enqueue(runtimeObj)
}

// OnUpdate handles object update event and push the object to queue.
func (c *controller) OnUpdate(oldObj, newObj interface{}) {
	c.OnAdd(newObj)
}

// OnDelete handles object delete event and push the object to queue.
func (c *controller) OnDelete(obj interface{}) {
	c.OnAdd(obj)
}

func (c *controller) EventFilter(obj interface{}) bool {
	//todo
	return true
}
