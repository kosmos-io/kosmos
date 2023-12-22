package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"

	"github.com/kosmos.io/kosmos/cmd/webhook/app"
)

func main() {
	ctx := apiserver.SetupSignalContext()
	cmd := app.NewWebhookCommand(ctx)
	code := cli.Run(cmd)
	os.Exit(code)
}
