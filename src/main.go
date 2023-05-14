package main

import (
	"log"

	a "github.com/vault-thirteen/Forum-Crawler/src/pkg/App"
	"github.com/vault-thirteen/Forum-Crawler/src/pkg/CLIArguments"
)

func mustBeNoError(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	var err error
	var cliArgs *cli.Arguments
	cliArgs, err = cli.NewArguments()
	mustBeNoError(err)

	var app *a.App
	app, err = a.NewApp(cliArgs)
	mustBeNoError(err)
	defer func() {
		derr := app.Close()
		if derr != nil {
			log.Println(derr)
		}
	}()
}
