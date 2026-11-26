package node

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadRolePlanPreservesFlagsAndInputBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan")
	limit := bytes.Repeat([]byte("x"), 16<<10)
	if err := os.WriteFile(path, limit, 0600); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--config", "--retained-config"} {
		raw, retained, err := ReadRolePlan([]string{flag, path})
		if err != nil || retained != (flag == "--retained-config") || !bytes.Equal(raw, limit) {
			t.Fatalf("%s changed the declaration or retained mode: retained=%v err=%v", flag, retained, err)
		}
	}
	if err := os.WriteFile(path, append(limit, 'x'), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadRolePlan([]string{"--config", path}); err == nil {
		t.Fatal("oversized declaration accepted")
	}
	if _, _, err := ReadRolePlan([]string{"--unknown", path}); err == nil {
		t.Fatal("unknown declaration flag accepted")
	}
}

func TestCanceledNodeRoleRefusesBeforeConfiguration(t *testing.T) {
	for _, role := range []string{"descriptor", "introduction", "join"} {
		for _, flag := range []string{"--config", "--retained-config"} {
			t.Run(role+"/"+flag, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				path := filepath.Join(t.TempDir(), "unavailable-private-input")
				var out bytes.Buffer
				raw, retained, code := ReadCancelableRolePlan(ctx, "node."+role, []string{flag, path}, &out)
				if code != 130 || raw != nil || retained {
					t.Fatal("already-canceled Node command read config", code, out.String())
				}
				var event map[string]string
				if err := json.Unmarshal(out.Bytes(), &event); err != nil || len(event) != 3 || event["operation"] != "node."+role || event["phase"] != "refused" || event["outcome"] != "canceled" {
					t.Fatal("canceled Node command minted readiness", event, err)
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("canceled command created input", err)
				}
			})
		}
	}
}
