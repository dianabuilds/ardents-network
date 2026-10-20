//go:build linux

package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
)

func TestNetworkSigningInitialKeyPlan(t *testing.T) {
	for _, count := range []string{"1", "64"} {
		if _, e := admitInitialKeyPlan([]byte(`{"schema":"ardents-initial-keys-plan-v1","operation":"prepare-initial-keys","node_count":` + count + `}`)); e != nil {
			t.Fatal(e)
		}
	}
	for _, count := range []string{"0", "65", "-1", "1.0", "1e0", "null", "true", `"1"`, "18446744073709551616"} {
		if _, e := admitInitialKeyPlan([]byte(`{"schema":"ardents-initial-keys-plan-v1","operation":"prepare-initial-keys","node_count":` + count + `}`)); e == nil {
			t.Fatal("count accepted", count)
		}
	}
	for _, raw := range []string{
		`{}`, `null`, `[]`,
		`{"schema":"ardents-initial-keys-plan-v1","operation":"prepare-initial-keys","node_count":1,"node_count":2}`,
		`{"schema":"ardents-initial-keys-plan-v1","operation":"prepare-initial-keys","node_count":1,"seed":"x"}`,
		`{"schema":"ardents-initial-keys-plan-v1","operation":"prepare-initial-keys","node_count":1} {}`,
		`{"schema":"ardents-initial-keys-plan-v1","operation":"sign-initial-epoch","node_count":1}`,
		`{"schema":"ardents-qualification-network-key-receipt-v1","operation":"prepare-initial-keys","node_count":1}`,
	} {
		if _, e := admitInitialKeyPlan([]byte(raw)); e == nil {
			t.Fatal("declaration accepted", raw)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	parent := t.TempDir()
	if c := runInitialKeys(ctx, []string{"prepare-initial-keys", filepath.Join(parent, "plan"), filepath.Join(parent, "destination")}, &bytes.Buffer{}); c != 130 {
		t.Fatal(c)
	}
	if c := runInitialKeys(context.Background(), []string{"prepare-initial-keys", "relative", "/destination"}, &bytes.Buffer{}); c != 2 {
		t.Fatal(c)
	}
}
