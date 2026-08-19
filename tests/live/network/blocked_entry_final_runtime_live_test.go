//go:build live

package network_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

type finalBridgeRuntime struct {
	Schema   string          `json:"schema"`
	Ordinal  uint16          `json:"ordinal"`
	Resource resource.Sample `json:"resource"`
}

func writeFinalBridgeRuntime(t *testing.T, root string, samples []resource.Sample) {
	t.Helper()
	if len(samples) == 0 {
		t.Fatal("Bridge runtime stream is empty")
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	for index, sample := range samples {
		if err := encoder.Encode(finalBridgeRuntime{Schema: "ardents-h3-bridge-runtime-v1",
			Ordinal: uint16(index), Resource: sample}); err != nil {
			t.Fatal(err)
		}
	}
	if output.Len() > maximumFinalTelemetryFile {
		t.Fatal("Bridge runtime stream is oversized")
	}
	if err := os.WriteFile(filepath.Join(root, "sync", "bridge", "runtime.jsonl"), output.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}
