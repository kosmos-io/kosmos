package adaper

import (
	"time"

	"github.com/projectcalico/calico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/calico/libcalico-go/lib/backend/model"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	clusterlister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
)

// syncedPollPeriod controls how often you look at the status of your sync funcs
var syncedPollPeriod = 100 * time.Millisecond

type BlockEventHandler struct {
	// Channel for getting updates and status updates from syncer.
	syncerC chan interface{}

	processor         lifted.AsyncWorker
	clusterNodeLister clusterlister.ClusterNodeLister
	// Channel that notifies the Run method to exit
	done chan struct{}

	// Flag to show we are in-sync.
	inSync bool
}

func NewBlockEventHandler(processor lifted.AsyncWorker, clusterNodeLister clusterlister.ClusterNodeLister) *BlockEventHandler {
	return &BlockEventHandler{
		processor:         processor,
		clusterNodeLister: clusterNodeLister,
		syncerC:           make(chan interface{}, 100),
		done:              make(chan struct{}),
	}
}

func (b *BlockEventHandler) Run(stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		case <-b.done:
			return
		case event := <-b.syncerC:
			switch event := event.(type) {
			case []api.Update:
				b.onupdate(event)
			case api.SyncStatus:
				b.inSync = true
			}
		}
	}
}

func (b *BlockEventHandler) Stop() {
	b.done <- struct{}{}
}

func (b *BlockEventHandler) Done() <-chan struct{} {
	return b.done
}

func (b *BlockEventHandler) InSync() bool {
	return b.inSync
}

func (b *BlockEventHandler) OnStatusUpdated(status api.SyncStatus) {
	if status == api.InSync {
		b.syncerC <- status
	}
}

func (b *BlockEventHandler) OnUpdates(updates []api.Update) {
	b.syncerC <- updates
}

func (b *BlockEventHandler) onupdate(event []api.Update) {
	klog.V(7).Info("update event")

	for _, update := range event {
		blockAffinityKey, ok := update.Key.(model.BlockAffinityKey)
		if !ok {
			log.Errorf("Failed to cast object to blockAffinityKey: %+v", update.Key)
			return
		}

		node := blockAffinityKey.Host
		requeue(node, b.clusterNodeLister, b.processor)
		klog.V(4).Infof("Processing blockAffinityKey update: %+v", update.Key)
	}
}

func (b *BlockEventHandler) WaitForCacheSync(stopCh <-chan struct{}) bool {
	err := wait.PollImmediateUntil(syncedPollPeriod, func() (done bool, err error) {
		if b.inSync {
			return true, nil
		}
		return false, nil
	}, stopCh)

	if err != nil {
		klog.V(2).Infof("stop requested")
		return false
	}

	klog.V(4).Infof("caches populated")
	return true
}
