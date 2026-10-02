//go:build !linux

package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestIssuerProfileCommandsRefuseBeforeReadingOnOtherPlatforms(t *testing.T) {
	for _, command := range [][]string{{"node-identity", "import"}, {"issuance", "initialize-profile"}, {"issuance", "inspect-profile"}, {"admission", "prepare-binding"}} {
		args := append(command, "--config", "missing-private-input")
		var out, log bytes.Buffer
		code := run(t.Context(), args, &out, &log)
		var result issuanceResult
		if json.Unmarshal(out.Bytes(), &result) != nil || code != 1 || result.Outcome != "unsupported-platform" {
			t.Fatal(code, &out, &log)
		}
	}
}
