package adaper

import (
	"context"
	"fmt"
	"strings"

	"github.com/projectcalico/calico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/calico/libcalico-go/lib/backend/model"
	"k8s.io/klog/v2"

	clusterlister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
)

type CalicoETCDAdapter struct {
	sync              bool
	watchSyncer       api.Syncer
	etcdClient        api.Client
	clusterNodeLister clusterlister.ClusterNodeLister
	processor         lifted.AsyncWorker
}

// nolint:revive
func NewCalicoETCDAdapter(etcdClient api.Client,
	clusterNodeLister clusterlister.ClusterNodeLister,
	processor lifted.AsyncWorker) *CalicoETCDAdapter {
	return &CalicoETCDAdapter{
		etcdClient:        etcdClient,
		clusterNodeLister: clusterNodeLister,
		processor:         processor,
	}
}

func (c *CalicoETCDAdapter) Start(stopCh <-chan struct{}) error {
	blockEventHandler := NewBlockEventHandler(c.processor, c.clusterNodeLister)
	c.watchSyncer = NewBlockWatchSyncer(c.etcdClient, blockEventHandler)
	c.watchSyncer.Start()
	go blockEventHandler.Run(stopCh)

	blockEventHandler.WaitForCacheSync(stopCh)
	c.sync = true
	klog.Info("calico blockaffinities etcd watchsyncer started!")
	return nil
}

func (c *CalicoETCDAdapter) GetCIDRByNodeName(nodeName string) ([]string, error) {
	var podCIDRS []string

	ctx := context.Background()

	blockAffinityKVList, err := c.etcdClient.List(ctx, model.BlockAffinityListOptions{}, "")
	if err != nil {
		return nil, err
	}

	for _, item := range blockAffinityKVList.KVPairs {
		etcdBlockAffinityKey, ok := item.Key.(model.BlockAffinityKey)
		if !ok {
			return nil, fmt.Errorf("error converting Key to BlockAffinityKey: %+v", item.Key)
		}

		if strings.Compare(etcdBlockAffinityKey.Host, nodeName) == 0 {
			podCIDRS = append(podCIDRS, etcdBlockAffinityKey.CIDR.String())
			klog.V(4).Infof("BlockAffinityKey %v CIDR appended, nodeName: %s", etcdBlockAffinityKey, nodeName)
		}
	}

	return podCIDRS, nil
}

func (c *CalicoETCDAdapter) Synced() bool {
	return c.sync
}
