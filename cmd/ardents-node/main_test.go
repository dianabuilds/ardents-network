package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceModeRejectsIncompleteInvocation(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{nil, {"source"}, {"source", "--config"}, {"role", "--config", "x"}} {
		if err := run(context.Background(), arguments, new(bytes.Buffer)); err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Fatalf("arguments %v returned %v", arguments, err)
		}
	}
}

func TestNodeOwnedPlansRejectRetiredH3Schemas(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		raw  string
		read func(string) error
	}{
		{"source-server", `{"schema":"ardents-h3-source-server-v1"}`, func(path string) error { _, err := openSource(path, nil); return err }},
		{"node", `{"schema":"ardents-h3-node-plan-v1"}`, func(path string) error { _, err := readNodePlan(path); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "retired-plan.json")
			if err := os.WriteFile(path, []byte(test.raw), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := test.read(path); err == nil {
				t.Fatalf("retired H3 %s plan schema was accepted", test.name)
			}
		})
	}
}
