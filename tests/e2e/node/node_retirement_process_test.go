package state_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRetiredNodeSelectorsRefuseBeforeRootEffects(t *testing.T) {
	binary := buildCommand(t, "ardents-node")
	root := t.TempDir()
	path := filepath.Join(root, "old-config.json")
	plan := []byte(`{"schema":"ardents-node-plan-v1","state_root":"retained"}`)
	if err := os.WriteFile(path, plan, 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		arguments []string
		refusal   string
	}{
		{[]string{"node", "--config"}, "old Node --config runtime is retired; use ardents-next node <role> --retained-config"},
		{[]string{"issuer", "initialize", "--config"}, "old Node issuer initialize/serve is retired; use ardents-next node-identity/issuance/node issuer"},
		{[]string{"issuer", "serve", "--config"}, "old Node issuer initialize/serve is retired; use ardents-next node-identity/issuance/node issuer"},
		{[]string{"source", "--config"}, "old Source profile is retired"},
	}
	for _, test := range tests {
		for _, input := range []string{path, filepath.Join(root, "absent.json")} {
			arguments := append(append([]string{}, test.arguments...), input)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			command := exec.CommandContext(ctx, binary, arguments...)
			command.Dir = root
			output, commandErr := command.CombinedOutput()
			cancel()
			if commandErr == nil || string(output) != test.refusal+"\n" {
				t.Fatalf("%v: err=%v output=%q", test.arguments, commandErr, output)
			}
		}
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, plan) {
		t.Fatalf("retired selectors changed the original configuration: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("retired selectors changed the input directory: %v / %v", entries, err)
	}
}
