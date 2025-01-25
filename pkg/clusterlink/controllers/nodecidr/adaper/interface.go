package adaper

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	clusterlister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils/keys"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
)

type CniAdapter interface {
	GetCIDRByNodeName(nodeName string) ([]string, error)

	Start(stopCh <-chan struct{}) error

	Synced() bool
}

func requeue(originNodeName string, clusterNodeLister clusterlister.ClusterNodeLister, processor lifted.AsyncWorker) {
	clusterNodes, err := clusterNodeLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("list clusterNodes err: %v", err)
		return
	}

	flag := false
	for _, clusterNode := range clusterNodes {
		if clusterNode.Spec.NodeName == originNodeName {
			key, err := ClusterWideKeyFunc(clusterNode)
			if err != nil {
				klog.Errorf("make clusterNode as a reconsile key err: %v", err)
				return
			}

			klog.V(7).Infof("key %s is enqueued!", originNodeName)
			processor.Add(key)
			flag = true
			break
		}
	}
	if !flag {
		clusterNode := &clusterlinkv1alpha1.ClusterNode{
			ObjectMeta: metav1.ObjectMeta{
				Name: originNodeName,
			},
		}
		key, err := ClusterWideKeyFunc(clusterNode)
		if err != nil {
			klog.Errorf("make clusterNode as a reconsile key err: %v", err)
			return
		}

		klog.V(7).Infof("can't find match clusternode %s", originNodeName)
		processor.Add(key)
	}
}

// ClusterWideKeyFunc generates a ClusterWideKey for object.
func ClusterWideKeyFunc(obj interface{}) (lifted.QueueKey, error) {
	return keys.ClusterWideKeyFunc(obj)
}
