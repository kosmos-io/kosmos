package main

import (
	"math/rand"
	"os"
	"time"

	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	_ "github.com/kosmos.io/kosmos/pkg/apis/config/scheme"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/plugins/leafnodedistribution"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/plugins/leafnodetainttoleration"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/plugins/leafnodevolumebinding"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/plugins/leafnodeworkloadpolicy"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	// Register custom plugins to the scheduler framework.
	// Later they can consist of scheduler profile(s) and hence
	// used by various kinds of workloads.
	command := app.NewSchedulerCommand(
		app.WithPlugin(leafnodedistribution.Name, leafnodedistribution.New),
		app.WithPlugin(leafnodetainttoleration.Name, leafnodetainttoleration.New),
		app.WithPlugin(leafnodevolumebinding.Name, leafnodevolumebinding.New),
		app.WithPlugin(leafnodeworkloadpolicy.Name, leafnodeworkloadpolicy.New),
	)

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
