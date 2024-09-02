package main

import (
	"os"

	"k8s.io/component-base/cli"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/kosmos/cmd/kubenest/ironic-param-syncer/app"
)

func main() {
	ctx := ctrl.SetupSignalHandler()
	cmd := app.NewIronicParameterSyncerCommand(ctx)
	code := cli.Run(cmd)
	os.Exit(code)
}
