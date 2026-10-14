//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCanceledDescriptorCommandRefusesBeforeConfiguration(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	path := filepath.Join(t.TempDir(), "unavailable-private-input")
	var out, diagnostic bytes.Buffer
	if code := run(ctx, []string{"node", "descriptor", "--config", path}, &out, &diagnostic); code != 130 {
		t.Fatal("already-canceled Descriptor command touched config", code, out.String(), diagnostic.String())
	}
	var event map[string]string
	if err := json.Unmarshal(out.Bytes(), &event); err != nil || len(event) != 3 || event["operation"] != "node.descriptor" || event["phase"] != "refused" || event["outcome"] != "canceled" {
		t.Fatal("canceled Descriptor command minted readiness", event, err)
	}
	if diagnostic.Len() != 0 {
		t.Fatal("canceled command disclosed private cause")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("canceled command created input", err)
	}
}
