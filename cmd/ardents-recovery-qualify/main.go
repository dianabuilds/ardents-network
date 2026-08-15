package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

const maximumEvidence = 52 << 20

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, output, diagnostics io.Writer) int {
	if len(arguments) != 1 {
		fmt.Fprintln(diagnostics, "usage: ardents-recovery-qualify <evidence.json>")
		return 2
	}
	file, err := os.Open(arguments[0])
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > maximumEvidence {
		fmt.Fprintln(diagnostics, "recovery evidence exceeds 52 MiB")
		return 2
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximumEvidence+1))
	if err != nil || len(raw) > maximumEvidence {
		fmt.Fprintln(diagnostics, "recovery evidence read is invalid or oversized")
		return 2
	}
	var evidence recovery.Evidence
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&evidence); err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		fmt.Fprintln(diagnostics, "recovery evidence has trailing JSON")
		return 2
	}
	result := recovery.Verify(evidence)
	if err := json.NewEncoder(output).Encode(result); err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	switch result.Verdict {
	case "pass":
		return 0
	case "fail":
		return 1
	default:
		return 2
	}
}
