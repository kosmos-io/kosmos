package blockwatchsyncer

import (
	"time"

	"github.com/projectcalico/calico/libcalico-go/lib/backend/api"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
)

// syncedPollPeriod controls how often you look at the status of your sync funcs
var syncedPollPeriod = 100 * time.Millisecond

type BlockEventHandler struct {
	// Channel for getting updates and status updates from syncer.
	syncerC chan interface{}

	processor lifted.AsyncWorker
	// Channel to indicate node status reporter routine is not needed anymore.
	done chan struct{}

	// Flag to show we are in-sync.
	inSync bool
}

func NewBlockEventHandler(processor lifted.AsyncWorker) *BlockEventHandler {
	return &BlockEventHandler{
		processor: processor,
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

// todo put etcd's event info AsyncWorker's queue
func (b *BlockEventHandler) onupdate(_ []api.Update) {

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
