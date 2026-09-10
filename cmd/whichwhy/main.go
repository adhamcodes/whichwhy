package main

import (
	"os"

	"github.com/adhamcodes/whichwhy/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, version))
}
