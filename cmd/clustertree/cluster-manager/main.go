package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"
	"k8s.io/klog"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app"
)

func main() {
	ctx := apiserver.SetupSignalContext()
	cmd, err := app.NewAgentCommand(ctx)
	if err != nil {
		klog.Errorf("error happened when new agent command, err: %v", err)
	}
	code := cli.Run(cmd)
	os.Exit(code)
}
