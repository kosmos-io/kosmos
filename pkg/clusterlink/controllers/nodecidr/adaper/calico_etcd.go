package adaper

import (
	clusterlister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
	"github.com/projectcalico/calico/libcalico-go/lib/backend/api"
)

type CalicoETCDAdapter struct {
	sync              bool
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
	// todo use c.etcdClient to list and watch blockaffinity in etcd
	return nil
}

func (c *CalicoETCDAdapter) GetCIDRByNodeName(nodeName string) ([]string, error) {
	// see calicoctl/calicoctl/commands/datastore/migrate/migrateipam.go
	// and libcalico-go/lib/backend/model/block_affinity.go
	// todo use c.etcdClient to get blockaffinity in etcd
	return nil, nil
}

func (c *CalicoETCDAdapter) Synced() bool {
	return c.sync
}

func (c *CalicoETCDAdapter) OnAdd(obj interface{}) {
	// todo add event info to c.processor
}

// OnUpdate handles object update event and push the object to queue.
func (c *CalicoETCDAdapter) OnUpdate(_, newObj interface{}) {
	// todo add event info to c.processor
}

// OnDelete handles object delete event and push the object to queue.
func (c *CalicoETCDAdapter) OnDelete(obj interface{}) {
	// todo add event info to c.processor
}
