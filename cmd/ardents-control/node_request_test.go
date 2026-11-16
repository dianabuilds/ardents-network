package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOtherNodeRequestArgumentsRemainClosedDeclarations(t *testing.T) {
	for _, role := range []struct {
		name  string
		parse func([]string) ([]byte, string, error)
	}{
		{"forwarding", forwardingRequestArguments},
		{"introduction", introductionRequestArguments},
	} {
		t.Run(role.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "output")
			args := []string{"--installation-root", "/opt/ardents-node", "--state-root", "/var/lib/node-state", "--participation-root", "/var/lib/node-participation", "--spend-root", "/var/lib/node-spend", "--hosting-root", "/var/lib/node-hosting", "--credential-directory", "/etc/ardents-node", "--plan", "/var/lib/delivery/plan.json", "--certificate", "/var/lib/delivery/certificate.pem", "--private-key", "/var/lib/delivery/key.pem", "--clock", "/var/lib/node-clock/observation"}
			if role.name == "introduction" {
				args = append(args, "--introduction-root", "/var/lib/node-introduction")
			}
			args = append(args, "--output", output)
			raw, path, err := role.parse(args)
			var document map[string]any
			if err != nil || path != output || json.Unmarshal(raw, &document) != nil || document["invocation"] != role.name {
				t.Fatal("valid declaration refused", path, err)
			}
			if _, err := os.Lstat(output); !os.IsNotExist(err) {
				t.Fatal("declaration wrote output", err)
			}
			args[len(args)-1] = "/var/lib/node-hosting"
			if _, _, err := role.parse(args); err == nil {
				t.Fatal("output overlapped a semantic root")
			}
		})
	}
}
