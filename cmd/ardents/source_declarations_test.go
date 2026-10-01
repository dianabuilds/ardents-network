package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceIdentityRefusalPrecedesCredentialFileRead(t *testing.T) {
	root := t.TempDir()
	plan := sourcePlan{Schema: "ardents-source-plan-v1", NetworkID: strings.Repeat("11", 32),
		AuthorityPublic: []string{strings.Repeat("aa", 32)}, Threshold: 1,
		ClockObservedAt: "2026-09-30T12:00:00Z", OrderSeed: strings.Repeat("22", 32),
		LocalRoleStateRoot: filepath.Join(root, "role"), ClientCertificate: filepath.Join(root, "absent-cert"),
		ClientKey: filepath.Join(root, "absent-key")}
	for _, identity := range []string{"33", "44"} {
		plan.Sources = append(plan.Sources, sourcePlanMember{Identity: strings.Repeat(identity, 32),
			LeafKeyDigest: strings.Repeat("55", 32), RootCA: filepath.Join(root, "absent-ca")})
	}
	path := filepath.Join(root, "plan.json")
	write := func() {
		t.Helper()
		raw, err := json.Marshal(plan)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	if _, err := readSourcePlan(filepath.Join(root, "state"), path); !os.IsNotExist(err) {
		t.Fatalf("valid declarations must reach the missing credential file: %v", err)
	}
	plan.Sources[1].LeafKeyDigest = "55"
	write()
	if _, err := readSourcePlan(filepath.Join(root, "state"), path); err == nil ||
		!strings.Contains(err.Error(), "invalid fixed hexadecimal value") || os.IsNotExist(err) {
		t.Fatalf("invalid Source digest must refuse before missing credential access: %v", err)
	}
	for _, name := range []string{"state", "role", "absent-cert", "absent-key", "absent-ca"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("declaration/refusal path created %s: %v", name, err)
		}
	}
}
