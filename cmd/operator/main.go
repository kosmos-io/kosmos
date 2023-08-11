package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"

	"github.com/kosmos.io/clusterlink/cmd/operator/app"
)

func main() {
	ctx := apiserver.SetupSignalContext()
	cmd := app.NewOperatorCommand(ctx)
	code := cli.Run(cmd)
	os.Exit(code)
}
