package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app"
	_ "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager"
	_ "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers"
	_ "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/mcs"
	_ "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod"
	_ "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pv"
	_ "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pvc"
)

func main() {
	ctx := apiserver.SetupSignalContext()
	cmd := app.NewAgentCommand(ctx)
	code := cli.Run(cmd)
	os.Exit(code)
}
