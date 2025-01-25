package adaper

import (
	corev1 "k8s.io/api/core/v1"
	informer "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	clusterlister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
)

type CommonAdapter struct {
	sync              bool
	config            *rest.Config
	nodeLister        lister.NodeLister
	clusterNodeLister clusterlister.ClusterNodeLister
	processor         lifted.AsyncWorker
}

// nolint:revive
func NewCommonAdapter(config *rest.Config,
	clusterNodeLister clusterlister.ClusterNodeLister,
	processor lifted.AsyncWorker) *CommonAdapter {
	return &CommonAdapter{
		config:            config,
		clusterNodeLister: clusterNodeLister,
		processor:         processor,
	}
}

func (c *CommonAdapter) Start(stopCh <-chan struct{}) error {
	client, err := kubernetes.NewForConfig(c.config)
	if err != nil {
		return err
	}

	informerFactory := informer.NewSharedInformerFactory(client, 0)
	c.nodeLister = informerFactory.Core().V1().Nodes().Lister()
	_, err = informerFactory.Core().V1().Nodes().Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.OnAdd,
			UpdateFunc: c.OnUpdate,
			DeleteFunc: c.OnDelete,
		})
	if err != nil {
		return err
	}

	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	c.sync = true
	klog.Info("common informer started!")
	return nil
}

func (c *CommonAdapter) GetCIDRByNodeName(nodeName string) ([]string, error) {
	node, err := c.nodeLister.Get(nodeName)
	if err != nil {
		klog.Infof("get node %s error:%v", nodeName, err)
		return nil, err
	}

	return node.Spec.PodCIDRs, nil
}

func (c *CommonAdapter) Synced() bool {
	return c.sync
}

func (c *CommonAdapter) OnAdd(obj interface{}) {
	runtimeObj, ok := obj.(*corev1.Node)
	if !ok {
		return
	}
	requeue(runtimeObj.Name, c.clusterNodeLister, c.processor)
}

// OnUpdate handles object update event and push the object to queue.
func (c *CommonAdapter) OnUpdate(_, newObj interface{}) {
	runtimeObj, ok := newObj.(*corev1.Node)
	if !ok {
		return
	}
	requeue(runtimeObj.Name, c.clusterNodeLister, c.processor)
}

// OnDelete handles object delete event and push the object to queue.
func (c *CommonAdapter) OnDelete(obj interface{}) {
	runtimeObj, ok := obj.(*corev1.Node)
	if !ok {
		return
	}
	requeue(runtimeObj.Name, c.clusterNodeLister, c.processor)
}
