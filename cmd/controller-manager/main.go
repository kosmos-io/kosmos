package main

import (
	"os"

	"k8s.io/component-base/cli"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/clusterlink/cmd/controller-manager/app"
)

func main() {
	ctx := ctrl.SetupSignalHandler()
	cmd := app.NewControllerManagerCommand(ctx)
	code := cli.Run(cmd)
	os.Exit(code)
}
