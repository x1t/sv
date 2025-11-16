package main

import (
	"os"

	"github.com/x1t/sv/pkg/cli"
)









func main() {
	app := cli.NewCLIApp()
	err := app.Run()
	if err != nil {
		os.Exit(1)
	}
}