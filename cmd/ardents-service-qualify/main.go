package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	qualification "github.com/dianabuilds/ardents-network/internal/qualification/service"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) != 1 || arguments[0] == "" {
		return errors.New("usage: ardents-service-qualify <evidence.json>")
	}
	file, err := os.Open(arguments[0])
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(file, 4<<20+1))
	_ = file.Close()
	if err != nil {
		return err
	}
	verdict := qualification.Verify(raw)
	if err := json.NewEncoder(output).Encode(verdict); err != nil {
		return err
	}
	if verdict.Verdict != "pass" {
		return errors.New(verdict.Verdict)
	}
	return nil
}
