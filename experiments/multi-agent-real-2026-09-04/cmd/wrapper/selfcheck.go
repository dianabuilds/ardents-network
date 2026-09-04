//go:build ignore

package main

import (
	"errors"
	"fmt"
)

func runSelfTest() error {
	if _, err := decodeAction([]byte(`{"schema":"ardents-agent-action-v1","action":"refresh"}`)); err != nil {
		return fmt.Errorf("S1 valid action: %w", err)
	}
	if _, err := decodeAction([]byte(`{"schema":"ardents-agent-action-v1","action":"refresh","state_root":"tick-1"}`)); err == nil {
		return errors.New("S2 path injection was accepted")
	}
	for message, expected := range map[string]string{
		"local role record limit exceeded":                                        "record_limit",
		"local role producer limit exceeded":                                      "producer_limit",
		"local role duty conflicts with retained state":                           "conflict",
		"direct-source exposure set is full":                                      "installation_source_exhausted",
		"finite sources are temporarily unavailable: retry is in durable backoff": "durable_backoff",
		"durable source cycle reached its recorded deadline":                      "cycle_deadline",
		"docker: Error response from daemon: No such container":                   "container_missing",
	} {
		_, diagnostic := classifyFailure(message)
		if diagnostic != expected {
			return fmt.Errorf("S3 classify %q = %q, want %q", message, diagnostic, expected)
		}
	}
	return nil
}
