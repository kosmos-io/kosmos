package main

import (
	"os"

	"k8s.io/component-base/cli"

	"github.com/kosmos.io/kosmos/cmd/koslet/app"
)

func main() {
	command := app.NewKosletCommand()
	code := cli.Run(command)
	os.Exit(code)
}
