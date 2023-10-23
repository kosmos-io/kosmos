package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"

	"github.com/kosmos.io/kosmos/cmd/koslet/app"
)

func main() {
	ctx := apiserver.SetupSignalContext()
	command := app.NewKosletCommand(ctx)
	code := cli.Run(command)
	os.Exit(code)
}
