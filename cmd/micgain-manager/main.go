package main

import (
	"fmt"
	"os"

	"micgain-manager/internal/adapter/primary/cli"
)

func main() {
	root := cli.NewRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
