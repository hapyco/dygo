package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dygo-dev/dygo/internal/cli"
)

func main() {
	if err := cli.Run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
