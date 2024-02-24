package main

import (
	"os"

	"k8s.io/component-base/cli"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	_ "github.com/kosmos.io/kosmos/pkg/apis/config/scheme"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/plugins/leafnodedistribution"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/plugins/leafnodetainttoleration"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/plugins/leafnodevolumebinding"
)

func main() {
	// Register custom plugins to the scheduler framework.
	// Later they can consist of scheduler profile(s) and hence
	// used by various kinds of workloads.
	command := app.NewSchedulerCommand(
		app.WithPlugin(leafnodedistribution.Name, leafnodedistribution.New),
		app.WithPlugin(leafnodetainttoleration.Name, leafnodetainttoleration.New),
		app.WithPlugin(leafnodevolumebinding.Name, leafnodevolumebinding.New),
	)
	code := cli.Run(command)
	os.Exit(code)
}
