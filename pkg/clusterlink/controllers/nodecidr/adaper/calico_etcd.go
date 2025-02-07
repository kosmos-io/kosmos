package adaper

import (
	"github.com/projectcalico/calico/libcalico-go/lib/backend/api"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clusterlink/controllers/nodecidr/adaper/blockwatchsyncer"
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
	blockEventHandler := blockwatchsyncer.NewBlockEventHandler(c.processor)
	c.watchSyncer = blockwatchsyncer.NewBlockWatchSyncer(c.etcdClient, blockEventHandler)
	c.watchSyncer.Start()
	blockEventHandler.Run(stopCh)

	blockEventHandler.WaitForCacheSync(stopCh)
	c.sync = true
	klog.Info("calico blockaffinities etcd watchsyncer started!")
	return nil
}

func (c *CalicoETCDAdapter) GetCIDRByNodeName(_ string) ([]string, error) {
	// see calicoctl/calicoctl/commands/datastore/migrate/migrateipam.go
	// and libcalico-go/lib/backend/model/block_affinity.go
	// todo use c.etcdClient to get blockaffinity in etcd
	return nil, nil
}

func (c *CalicoETCDAdapter) Synced() bool {
	return c.sync
}
