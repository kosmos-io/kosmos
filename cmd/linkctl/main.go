package main

import (
	"k8s.io/component-base/cli"
	"k8s.io/kubectl/pkg/cmd/util"

	app "github.com/kosmos.io/clusterlink/pkg/clusterlinkctl"
)

func main() {
	cmd := app.NewLinkCtlCommand("linkctl", "linkctl")
	if err := cli.RunNoErrOutput(cmd); err != nil {
		util.CheckErr(err)
	}
}
