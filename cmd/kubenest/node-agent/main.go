package main

import (
	"log"

	"github.com/kosmos.io/kosmos/cmd/kubenest/node-agent/app"
)

func main() {
	if err := app.RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
