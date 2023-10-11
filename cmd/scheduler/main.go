package main

import (
	"os"

	"k8s.io/component-base/cli"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	_ "github.com/kosmos.io/kosmos/pkg/apis/config/scheme"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/plugins/knodetainttoleration"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/plugins/knodevolumebinding"
)

func main() {
	// Register custom plugins to the scheduler framework.
	// Later they can consist of scheduler profile(s) and hence
	// used by various kinds of workloads.
	command := app.NewSchedulerCommand(
		app.WithPlugin(knodetainttoleration.Name, knodetainttoleration.New),
		app.WithPlugin(knodevolumebinding.Name, knodevolumebinding.New),
	)
	code := cli.Run(command)
	os.Exit(code)
}
