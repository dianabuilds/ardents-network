package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/qualification/servicenegative"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) > 1 {
		return fmt.Errorf("usage: ardents-service-negative [recovery-case]")
	}
	result, runErr := servicenegative.Run(context.Background(), arguments...)
	if err := json.NewEncoder(output).Encode(result); err != nil {
		return err
	}
	return runErr
}
