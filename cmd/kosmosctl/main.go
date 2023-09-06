package main

import (
	"k8s.io/component-base/cli"
	"k8s.io/kubectl/pkg/cmd/util"

	app "github.com/kosmos.io/kosmos/pkg/kosmosctl"
)

func main() {
	cmd := app.NewKosmosCtlCommand()
	if err := cli.RunNoErrOutput(cmd); err != nil {
		util.CheckErr(err)
	}
}
