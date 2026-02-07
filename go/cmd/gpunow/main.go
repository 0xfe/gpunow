package main

import (
	"os"

	appcli "gpunow/internal/cli"
)

func main() {
	app := appcli.NewApp()
	args := appcli.NormalizeArgs(os.Args)
	if err := app.Run(args); err != nil {
		os.Exit(1)
	}
}
