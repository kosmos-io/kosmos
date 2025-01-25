package adaper

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	clusterlister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
)

type CalicoAdapter struct {
	sync              bool
	config            *rest.Config
	blockLister       cache.GenericLister
	clusterNodeLister clusterlister.ClusterNodeLister
	processor         lifted.AsyncWorker
}

// nolint:revive
func NewCalicoAdapter(config *rest.Config,
	clusterNodeLister clusterlister.ClusterNodeLister,
	processor lifted.AsyncWorker) *CalicoAdapter {
	return &CalicoAdapter{
		config:            config,
		clusterNodeLister: clusterNodeLister,
		processor:         processor,
	}
}

func (c *CalicoAdapter) Start(stopCh <-chan struct{}) error {
	client, err := dynamic.NewForConfig(c.config)
	if err != nil {
		klog.Errorf("init dynamic client err: %v", err)
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
		klog.Errorf("add event handler error: %v", err)
		return err
	}

	c.blockLister = informerFactory.ForResource(gvr).Lister()
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	c.sync = true
	klog.Info("calico blockaffinities informer started!")
	return nil
}

func (c *CalicoAdapter) GetCIDRByNodeName(nodeName string) ([]string, error) {
	blockAffinities, err := c.blockLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("list blockAffinities error: %v", err)
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

func (c *CalicoAdapter) Synced() bool {
	return c.sync
}

func (c *CalicoAdapter) OnAdd(obj interface{}) {
	klog.V(7).Info("add event")
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
	klog.V(7).Info("add event Enqueue")
	requeue(node, c.clusterNodeLister, c.processor)
}

// OnUpdate handles object update event and push the object to queue.
func (c *CalicoAdapter) OnUpdate(_, newObj interface{}) {
	klog.V(7).Info("update event")
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
	klog.V(7).Info("update event Enqueue")
	requeue(node, c.clusterNodeLister, c.processor)
}

// OnDelete handles object delete event and push the object to queue.
func (c *CalicoAdapter) OnDelete(obj interface{}) {
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
