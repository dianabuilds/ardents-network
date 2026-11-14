package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDescriptorRequestArgumentsAreClosedDeclarationsWithoutOutputEffects(t *testing.T) {
	output := filepath.Join(t.TempDir(), "output")
	args := []string{
		"--installation-root", "/opt/ardents-descriptor", "--state-root", "/var/lib/ardents-node-state",
		"--participation-root", "/var/lib/ardents-node-participation", "--spend-root", "/var/lib/ardents-node-spend",
		"--descriptor-root", "/var/lib/ardents-node-descriptor", "--hosting-root", "/var/lib/ardents-node-hosting", "--credential-directory", "/etc/ardents-node",
		"--plan", "/var/lib/ardents-delivery/descriptor.json", "--certificate", "/var/lib/ardents-delivery/certificate.pem",
		"--private-key", "/var/lib/ardents-delivery/private-key.pem", "--clock", "/var/lib/ardents-clock/observation", "--output", output,
	}
	raw, destination, err := descriptorRequestArguments(args)
	if err != nil || destination != output || len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatal("closed request preparation", destination, err)
	}
	var fields map[string]string
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields) != 14 || fields["schema"] != "ardents-descriptor-installation-v1" || fields["invocation"] != "descriptor" || fields["profile"] != "ardents-descriptor-process-v1" || fields["installation_root"] != "/opt/ardents-descriptor" || fields["clock"] != "/var/lib/ardents-clock/observation" {
		t.Fatal("independent declaration fields", fields, err)
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatal("declaration parser performed output effects", err)
	}
	variants := [][]string{
		args[:len(args)-1], append(append([]string{}, args...), "--state-root", "/var/lib/changed"),
		append(append([]string{}, args...), "--invocation", "forwarding"),
	}
	for _, invalid := range variants {
		if raw, destination, err := descriptorRequestArguments(invalid); err == nil || raw != nil || destination != "" {
			t.Fatal("incomplete, duplicate or foreign options supplied output", destination, err)
		}
	}
	for _, collision := range []string{"/opt/ardents-descriptor", "/opt/ardents-descriptor.descriptor-initial-journal", "/opt/ardents-descriptor.descriptor-retirement-00000063-journal", "/var/lib/ardents-node-hosting"} {
		invalid := append([]string{}, args...)
		invalid[len(invalid)-1] = collision
		if raw, destination, err := descriptorRequestArguments(invalid); err == nil || raw != nil || destination != "" {
			t.Fatal("preparation output occupied original lifetime namespace", collision, destination, err)
		}
	}
}
