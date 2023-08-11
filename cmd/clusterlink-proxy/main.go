package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"

	"github.com/kosmos.io/clusterlink/cmd/clusterlink-proxy/app"
)

func main() {
	ctx := apiserver.SetupSignalContext()
	cmd := app.NewClusterLinkProxyCommand(ctx)
	code := cli.Run(cmd)
	os.Exit(code)
}
