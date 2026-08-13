package main

import (
	"context"
	"fmt"
	"io"
	"os"

	nodequalification "github.com/dianabuilds/ardents-network/internal/qualification/node"
)

func runNodeOfflineCommand(arguments []string, output, diagnostics io.Writer) (int, bool) {
	handled, err := nodequalification.RunObserver(context.Background(), arguments, os.Stdin, output)
	if !handled {
		return 0, false
	}
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2, true
	}
	return 0, true
}
