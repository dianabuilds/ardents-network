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
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(output io.Writer) error {
	result, runErr := servicenegative.Run(context.Background())
	if err := json.NewEncoder(output).Encode(result); err != nil {
		return err
	}
	return runErr
}
