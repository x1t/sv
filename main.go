package main

import (
	"os"

	"sv/pkg/cli"
)









func main() {
	app := cli.NewCLIApp()
	err := app.Run()
	if err != nil {
		os.Exit(1)
	}
}