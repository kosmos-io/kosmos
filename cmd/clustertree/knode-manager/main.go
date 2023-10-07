package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"

	"github.com/kosmos.io/kosmos/cmd/clustertree/knode-manager/app"
)

func main() {
	ctx := apiserver.SetupSignalContext()
	command := app.NewKosmosNodeManagerCommand(ctx)
	code := cli.Run(command)
	os.Exit(code)
}
