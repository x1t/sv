package main

import (
	"fmt"
	"os"

	"github.com/x1t/sv/pkg/cli"
)

func main() {
	if err := cli.NewCLIApp().Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}
