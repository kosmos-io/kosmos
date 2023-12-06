package openapi

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	leafpodsyncers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod/leaf-pod"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/runtime"
)

func NewLeafPodOpenApiWorkerQueue(opts *leafpodsyncers.LeafPodWorkerQueueOption) runtime.Controller {
	// create the workqueue
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// launch go-routing to fetch data
	loopFunc := func(ctx context.Context) {
		// TODO: ctx
		err := opts.ServerlessClient.DoUpdate(func(eciName string) {
			queue.AddRateLimited(runtime.NamespacedName{
				Namespace: "",
				Name:      eciName,
			})
		})
		if err != nil {
			klog.Warningf("leaf loopFunc err: %s", err)
		}
	}

	// TODO: interval
	go wait.UntilWithContext(context.TODO(), loopFunc, 30*time.Second)

	leafOpenApiSyncer := &leafPodServerlessSyncer{
		ServerlessClient: opts.ServerlessClient,
		RootClient:       opts.RootClient,
	}

	return runtime.NewOpenApiWorkerQueue(queue, leafOpenApiSyncer)
}
