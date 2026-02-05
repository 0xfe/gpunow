package main

import (
	"os"

	appcli "gpunow/internal/cli"
)

func main() {
	app := appcli.NewApp()
	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}
