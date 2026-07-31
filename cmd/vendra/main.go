package main

import (
	"fmt"
	"os"

	"github.com/misaf/vendra-controller/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.New(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(4)
	}
}
