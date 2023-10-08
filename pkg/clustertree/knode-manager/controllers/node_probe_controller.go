package controllers

import (
	"context"
	"time"

	"golang.org/x/sync/singleflight"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/adapters"
	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/utils/lock"
)

const key = "key"
const DefaultProbeInterval = 10 * time.Second
const DefaultProbeTimeout = 5 * time.Second

type nodeProbeController struct {
	nodeAdapter adapters.NodeHandler
	interval    time.Duration
	timeout     time.Duration
	cond        lock.MonitorVariable
}

type probeResponse struct {
	time  time.Time
	error error
}

func newNodeProbeController(node adapters.NodeHandler) *nodeProbeController {
	return &nodeProbeController{
		nodeAdapter: node,
		interval:    DefaultProbeInterval,
		timeout:     DefaultProbeTimeout,
		cond:        lock.NewMonitorVariable(),
	}
}

func (npc *nodeProbeController) Run(ctx context.Context) {
	sf := &singleflight.Group{}

	mkContextFunc := func(ctx2 context.Context) (context.Context, context.CancelFunc) {
		return context.WithTimeout(ctx2, npc.timeout)
	}

	checkFunc := func(ctx context.Context) {
		ctx, cancel := mkContextFunc(ctx)
		defer cancel()

		doChan := sf.DoChan(key, func() (interface{}, error) {
			now := time.Now()
			err := npc.nodeAdapter.Probe(ctx)
			return now, err
		})

		var res probeResponse
		select {
		case <-ctx.Done():
			res.error = ctx.Err()
			klog.Warning("Failed to ping node due to context cancellation")
		case result := <-doChan:
			res.error = result.Err
			res.time = result.Val.(time.Time)
		}
		npc.cond.Set(&res)
	}

	checkFunc(ctx)
	wait.UntilWithContext(ctx, checkFunc, npc.interval)
}

func (npc *nodeProbeController) probe(ctx context.Context) (*probeResponse, error) {
	sub := npc.cond.Subscribe()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-sub.NewValueReady():
	}
	return sub.Value().Value.(*probeResponse), nil
}
