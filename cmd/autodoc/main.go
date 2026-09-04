package main

import (
	"fmt"
	"os"

	"github.com/pedro-dalben/autodoc/internal/cli"
)

func main() {
	root := cli.NewRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, cli.FormatError(err))
		os.Exit(1)
	}
}
