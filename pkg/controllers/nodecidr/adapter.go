package nodecidr

import (
	"strings"

	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	informer "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	clusterlister "github.com/kosmos.io/clusterlink/pkg/generated/listers/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/utils"
)

type cniAdapter interface {
	getCIDRByNodeName(nodeName string) ([]string, error)

	start(stopCh <-chan struct{}) error

	synced() bool
}

type commonAdapter struct {
	sync              bool
	config            *rest.Config
	nodeLister        lister.NodeLister
	clusterNodeLister clusterlister.ClusterNodeLister
	processor         utils.AsyncWorker
}

func NewCommonAdapter(config *rest.Config,
	clusterNodeLister clusterlister.ClusterNodeLister,
	processor utils.AsyncWorker) *commonAdapter {
	return &commonAdapter{
		config:            config,
		clusterNodeLister: clusterNodeLister,
		processor:         processor,
	}
}

func (c *commonAdapter) start(stopCh <-chan struct{}) error {
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
	return nil
}

func (c *commonAdapter) getCIDRByNodeName(nodeName string) ([]string, error) {
	node, err := c.nodeLister.Get(nodeName)
	if err != nil {
		return nil, err
	}

	return node.Spec.PodCIDRs, nil
}

func (c *commonAdapter) synced() bool {
	return c.sync
}

func (c *commonAdapter) OnAdd(obj interface{}) {
	runtimeObj, ok := obj.(api.Node)
	if !ok {
		return
	}
	requeue(runtimeObj.Name, c.clusterNodeLister, c.processor)
}

// OnUpdate handles object update event and push the object to queue.
func (c *commonAdapter) OnUpdate(oldObj, newObj interface{}) {
	runtimeObj, ok := newObj.(api.Node)
	if !ok {
		return
	}
	requeue(runtimeObj.Name, c.clusterNodeLister, c.processor)
}

// OnDelete handles object delete event and push the object to queue.
func (c *commonAdapter) OnDelete(obj interface{}) {
	runtimeObj, ok := obj.(api.Node)
	if !ok {
		return
	}
	requeue(runtimeObj.Name, c.clusterNodeLister, c.processor)
}

type calicoAdapter struct {
	sync              bool
	config            *rest.Config
	blockLister       cache.GenericLister
	clusterNodeLister clusterlister.ClusterNodeLister
	processor         utils.AsyncWorker
}

func NewCalicoAdapter(config *rest.Config,
	clusterNodeLister clusterlister.ClusterNodeLister,
	processor utils.AsyncWorker) *calicoAdapter {
	return &calicoAdapter{
		config:            config,
		clusterNodeLister: clusterNodeLister,
		processor:         processor,
	}
}

func (c *calicoAdapter) start(stopCh <-chan struct{}) error {
	client, err := dynamic.NewForConfig(c.config)
	if err != nil {
		return err
	}
	gvr := schema.GroupVersionResource{
		Group:    "crd.projectcalico.org",
		Version:  "v1",
		Resource: "blockaffinities",
	}
	informerFactory := dynamicinformer.NewDynamicSharedInformerFactory(client, 0)
	_, err = informerFactory.ForResource(gvr).Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.OnAdd,
			UpdateFunc: c.OnUpdate,
			DeleteFunc: c.OnDelete,
		})
	if err != nil {
		return err
	}

	c.blockLister = informerFactory.ForResource(gvr).Lister()
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	c.sync = true
	return nil
}

func (c *calicoAdapter) getCIDRByNodeName(nodeName string) ([]string, error) {
	blockAffinities, err := c.blockLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var podCIDRS []string
	for _, ba := range blockAffinities {
		uobj := ba.(*unstructured.Unstructured)
		node, found, err := unstructured.NestedString(uobj.Object, "spec", "node")
		if err != nil {
			klog.Errorf("get spec.node from blockAffinity err: ", err)
		}
		if !found {
			continue
		}
		cidr, found, err := unstructured.NestedString(uobj.Object, "spec", "cidr")
		if err != nil {
			klog.Errorf("get spec.cidr from blockAffinity err: ", err)
		}
		if !found {
			continue
		}
		if strings.Compare(node, nodeName) == 0 {
			podCIDRS = append(podCIDRS, cidr)
		}
	}

	return podCIDRS, nil
}

func (c *calicoAdapter) synced() bool {
	return c.sync
}

func (c *calicoAdapter) OnAdd(obj interface{}) {
	runtimeObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	node, found, err := unstructured.NestedString(runtimeObj.Object, "spec", "node")
	if err != nil {
		klog.Errorf("get spec.node from blockAffinity err: ", err)
	}
	if !found {
		return
	}
	requeue(node, c.clusterNodeLister, c.processor)
}

// OnUpdate handles object update event and push the object to queue.
func (c *calicoAdapter) OnUpdate(oldObj, newObj interface{}) {
	runtimeObj, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	node, found, err := unstructured.NestedString(runtimeObj.Object, "spec", "node")
	if err != nil {
		klog.Errorf("get spec.node from blockAffinity err: ", err)
	}
	if !found {
		return
	}
	requeue(node, c.clusterNodeLister, c.processor)
}

// OnDelete handles object delete event and push the object to queue.
func (c *calicoAdapter) OnDelete(obj interface{}) {
	runtimeObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	node, found, err := unstructured.NestedString(runtimeObj.Object, "spec", "node")
	if err != nil {
		klog.Errorf("get spec.node from blockAffinity err: ", err)
	}
	if !found {
		return
	}
	requeue(node, c.clusterNodeLister, c.processor)
}

func requeue(originNodeName string, clusterNodeLister clusterlister.ClusterNodeLister, processor utils.AsyncWorker) {
	clusterNodes, err := clusterNodeLister.List(labels.Everything())
	if err != nil {
		return
	}

	for _, clusterNode := range clusterNodes {
		if clusterNode.Spec.NodeName == originNodeName {
			key, err := ClusterWideKeyFunc(clusterNode)
			if err != nil {
				return
			}

			processor.Add(key)
			break
		}
	}
}
