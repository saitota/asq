package main

import (
	"fmt"
	"os"

	"github.com/saitota/asq/internal/cli"
)

func main() {
	if err := cli.New().Run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
