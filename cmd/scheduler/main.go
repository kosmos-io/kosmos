package main

import (
	"math/rand"
	"os"
	"time"

	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	_ "github.com/kosmos.io/kosmos/pkg/apis/config/scheme"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/plugins/leafnodetainttoleration"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/plugins/leafnodevolumebinding"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	// Register custom plugins to the scheduler framework.
	// Later they can consist of scheduler profile(s) and hence
	// used by various kinds of workloads.
	command := app.NewSchedulerCommand(
		app.WithPlugin(leafnodetainttoleration.Name, leafnodetainttoleration.New),
		app.WithPlugin(leafnodevolumebinding.Name, leafnodevolumebinding.New),
	)

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
