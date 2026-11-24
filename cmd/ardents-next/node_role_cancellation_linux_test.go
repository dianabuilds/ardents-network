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

func TestCanceledNodeRoleRefusesBeforeConfiguration(t *testing.T) {
	for _, role := range []string{"descriptor", "introduction", "join"} {
		for _, flag := range []string{"--config", "--retained-config"} {
			t.Run(role+"/"+flag, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				path := filepath.Join(t.TempDir(), "unavailable-private-input")
				var out, diagnostic bytes.Buffer
				if code := run(ctx, []string{"node", role, flag, path}, &out, &diagnostic); code != 130 {
					t.Fatal("already-canceled Node command touched config", code, out.String(), diagnostic.String())
				}
				var event map[string]string
				if err := json.Unmarshal(out.Bytes(), &event); err != nil || len(event) != 3 || event["operation"] != "node."+role || event["phase"] != "refused" || event["outcome"] != "canceled" {
					t.Fatal("canceled Node command minted readiness", event, err)
				}
				if diagnostic.Len() != 0 {
					t.Fatal("canceled command disclosed private cause")
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("canceled command created input", err)
				}
			})
		}
	}
}
