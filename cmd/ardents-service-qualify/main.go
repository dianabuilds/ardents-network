package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	qualification "github.com/dianabuilds/ardents-network/internal/qualification/service"
	"github.com/dianabuilds/ardents-network/internal/qualification/servicenegative"
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
	if receipt, negativeErr := servicenegative.Run(context.Background()); negativeErr != nil || len(receipt.Negatives) != 24 {
		verdict = qualification.Verdict{Schema: "ardents-h3-service-verdict-v1", Verdict: "invalid",
			Reason: "independent mandatory negative replay did not pass"}
	}
	if err := json.NewEncoder(output).Encode(verdict); err != nil {
		return err
	}
	if verdict.Verdict != "pass" {
		return errors.New(verdict.Verdict)
	}
	return nil
}
