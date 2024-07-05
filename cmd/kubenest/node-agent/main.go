package main

import (
	"github.com/kosmos.io/kosmos/cmd/kubenest/node-agent/app"
	"log"
)

func main() {
	if err := app.RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
