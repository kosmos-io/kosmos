package nodecidr

import (
	"context"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	informer "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	nodecontroller "github.com/kosmos.io/kosmos/pkg/clusterlink/controllers/node"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	clusterinformer "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions/kosmos/v1alpha1"
	clusterlister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils/keys"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
)

const (
	workNum     = 1
	requeueTime = 10 * time.Second
	calicoCNI   = "calico"
)

type Controller struct {
	clusterName string
	config      *rest.Config

	clusterLinkClient versioned.Interface
	nodeLister        lister.NodeLister

	// RateLimiterOptions is the configuration for rate limiter which may significantly influence the performance of
	// the Controller.
	RateLimiterOptions  lifted.RateLimitOptions
	processor           lifted.AsyncWorker
	clusterNodeInformer clusterinformer.ClusterNodeInformer
	clusterNodeLister   clusterlister.ClusterNodeLister

	cniAdapter cniAdapter
	sync.RWMutex
	ctx context.Context
}

func NewNodeCIDRController(context context.Context, config *rest.Config, clusterName string, clusterLinkClient versioned.Interface, RateLimiterOptions lifted.RateLimitOptions) *Controller {
	return &Controller{
		clusterLinkClient:  clusterLinkClient,
		config:             config,
		RateLimiterOptions: RateLimiterOptions,
		ctx:                context,
		clusterName:        clusterName,
	}
}

func (c *Controller) Start(ctx context.Context) error {
	klog.Infof("Starting node cidr Controller.")

	opt := lifted.WorkerOptions{
		Name:               "node cidr Controller",
		KeyFunc:            ClusterWideKeyFunc,
		ReconcileFunc:      c.Reconcile,
		RateLimiterOptions: c.RateLimiterOptions,
	}
	c.processor = lifted.NewAsyncWorker(opt)

	clusterInformerFactory := externalversions.NewSharedInformerFactory(c.clusterLinkClient, 0)

	c.clusterNodeInformer = clusterInformerFactory.Kosmos().V1alpha1().ClusterNodes()
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

	c.clusterNodeLister = clusterInformerFactory.Kosmos().V1alpha1().ClusterNodes().Lister()
	cluster, err := c.clusterLinkClient.KosmosV1alpha1().Clusters().Get(ctx, c.clusterName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("can not find local cluster %s, err: %v", c.clusterName, err)
		return err
	}

	client, err := kubernetes.NewForConfig(c.config)
	if err != nil {
		klog.Errorf("init kubernetes client err: %v", err)
		return err
	}

	informerFactory := informer.NewSharedInformerFactory(client, 0)
	c.nodeLister = informerFactory.Core().V1().Nodes().Lister()

	stopCh := c.ctx.Done()
	clusterInformerFactory.Start(stopCh)
	clusterInformerFactory.WaitForCacheSync(stopCh)

	// third step: init CNI Adapter
	if cluster.Spec.ClusterLinkOptions.CNI == calicoCNI {
		klog.Infof("cluster %s's cni is %s", c.clusterName, calicoCNI)
		c.cniAdapter = NewCalicoAdapter(c.config, c.clusterNodeLister, c.processor)
	} else {
		klog.Infof("cluster %s's cni is %s", c.clusterName, cluster.Spec.ClusterLinkOptions.CNI)
		c.cniAdapter = NewCommonAdapter(c.config, c.clusterNodeLister, c.processor)
	}
	err = c.cniAdapter.start(stopCh)
	if err != nil {
		return err
	}

	// last step : start processor and waiting for close chan
	c.processor.Run(workNum, stopCh)
	<-stopCh
	klog.Infof("Stop node cidr Controller as process done.")
	return nil
}

func (c *Controller) Reconcile(key lifted.QueueKey) error {
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Error("invalid key")
		return fmt.Errorf("invalid key")
	}
	klog.Infof("Reconciling Cluster Node: %s", clusterWideKey)
	clusterNode, err := c.clusterNodeLister.Get(clusterWideKey.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Info("maybe clusterWideKey.Name is k8s node's name instead of clusternode's name,try to get node podCIDRs")
			nodePodcidr, err := c.cniAdapter.getCIDRByNodeName(clusterWideKey.Name)
			// get cluster node name by clustername and k8s node's name
			clusterNodeName := nodecontroller.ClusterNodeName(c.clusterName, clusterWideKey.Name)
			// if err is no nil, means get node error or list blockAffinities error
			// do not reconsile
			if err != nil {
				klog.Errorf("get node %s's podCIDRs err: %v", clusterWideKey.Name, err)
				return err
			}
			// we execute this Reconcile func due to some node cidr event, like blockaffinities is created
			// so node podCIDRs should exist.If node podCIDRs is nil,maybe node is removed
			if len(nodePodcidr) == 0 {
				klog.Info("length of podCIDRs is 0 for node %s", clusterWideKey.Name)
				_, err := c.nodeLister.Get(clusterWideKey.Name)
				if err != nil {
					if apierrors.IsNotFound(err) {
						klog.Infof("k8s node %s is not found, might be removed.", clusterWideKey.Name)
						return nil
					}
					klog.Errorf("get node %s error:%v", clusterWideKey.Name, err)
					c.processor.AddAfter(key, requeueTime)
					return err
				}
			}
			// If k8s node exist, clusternode must exist
			clusterNode, err = c.clusterNodeLister.Get(clusterNodeName)
			if err != nil {
				klog.Infof("get clusternode %s err: %v", clusterNodeName, err)
				c.processor.AddAfter(key, requeueTime)
				return err
			}
		} else {
			klog.Errorf("get clusternode %s err : %v", clusterWideKey.Name, err)
			c.processor.AddAfter(key, requeueTime)
			return err
		}
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
		klog.Errorf("get node %s's podCIDRs err: %v", originNodeName, err)
		return err
	}

	clusterNodeCopy := clusterNode.DeepCopy()
	clusterNodeCopy.Spec.PodCIDRs = podCIDRs

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = c.clusterLinkClient.KosmosV1alpha1().ClusterNodes().Update(context.TODO(), clusterNodeCopy, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("update clusternode %s err: %v", clusterNodeCopy.Name, err)
			return err
		}
		klog.Infof("update clusternode %s succcessfuly", clusterNodeCopy.Name)
		return nil
	})
}

// ClusterWideKeyFunc generates a ClusterWideKey for object.
func ClusterWideKeyFunc(obj interface{}) (lifted.QueueKey, error) {
	return keys.ClusterWideKeyFunc(obj)
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
func (c *Controller) OnUpdate(_, newObj interface{}) {
	c.OnAdd(newObj)
}

// OnDelete handles object delete event and push the object to queue.
func (c *Controller) OnDelete(obj interface{}) {
	c.OnAdd(obj)
}

func (c *Controller) EventFilter(_ interface{}) bool {
	//todo
	return true
}
