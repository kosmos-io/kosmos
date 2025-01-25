package blockwatchsyncer

import (
	apiv3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	"github.com/projectcalico/calico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/calico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/calico/libcalico-go/lib/backend/watchersyncer"
)

// NewBlockWatchSyncer creates a new BlockAffinity v1 Syncer.
func NewBlockWatchSyncer(client api.Client, callbacks api.SyncerCallbacks) api.Syncer {
	resourceTypes := []watchersyncer.ResourceType{
		{
			ListInterface: model.ResourceListOptions{Kind: apiv3.KindBlockAffinity},
		},
	}

	return watchersyncer.New(client, resourceTypes, callbacks)
}
