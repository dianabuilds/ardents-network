//go:build !linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
)

// The non-native command stubs project the same explicit typed
// unsupported-platform refusal as the node-role stubs (ADR-0129 §3) on the
// command's own report writer, distinguishable from a usage error.
func TestCompiledCommandsRefuseUnsupportedPlatformExplicitly(t *testing.T) {
	t.Setenv("ARDENTS_DEBUG_SOCKET", "")
	cases := []struct {
		name      string
		args      []string
		operation string
		stderr    bool // refusal projects to diagnostic (stderr) rather than stdout
	}{
		{"platform", []string{"platform", "check", "/etc/ardents/unit"}, "platform", false},
		{"infrastructure", []string{"infrastructure", "prepare-runtime-inventory", "source", "/nonexistent-input", "/nonexistent-output"}, "infrastructure", false},
		{"source", []string{"source", "serve-retained", "/nonexistent-source"}, "source", true},
		{"source-repair", []string{"source", "repair-retained", "/nonexistent-source"}, "source.repair-retained", true},
		{"source-prepare-trust", []string{"source", "prepare-trust", "/nonexistent-a", "/nonexistent-b"}, "source.prepare-trust", false},
		{"route", []string{"route", "receive"}, "route", false},
		{"endpoint-start-installed", []string{"endpoint", "start-installed", "/nonexistent-socket"}, "endpoint", true},
		{"endpoint-prepare-history", []string{"endpoint", "prepare-history", "a", "b", "c"}, "endpoint", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			command := exec.CommandContext(t.Context(), compiledCommand(t), tc.args...)
			var out, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &out, &diagnostic
			err := command.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("unsupported %s lost the explicit refusal exit 1: %v", tc.name, err)
			}
			report, silent := &out, &diagnostic
			if tc.stderr {
				report, silent = &diagnostic, &out
			}
			assertUnsupportedPlatformRefusal(t, report, silent, tc.operation)
		})
	}
}

// assertUnsupportedPlatformRefusal checks the explicit typed refusal grammar on
// the command's report writer and that the other writer stayed silent.
func assertUnsupportedPlatformRefusal(t *testing.T, report, silent *bytes.Buffer, operation string) {
	t.Helper()
	var event map[string]string
	if json.Unmarshal(report.Bytes(), &event) != nil || len(event) != 3 ||
		event["operation"] != operation || event["phase"] != "refused" || event["outcome"] != "unsupported-platform" {
		t.Fatalf("missing typed unsupported-platform refusal for %s: %s", operation, report.String())
	}
	if silent.Len() != 0 {
		t.Fatalf("unsupported %s leaked output on the silent writer: %s", operation, silent.String())
	}
}

// An invalid installed-endpoint argument shape stays a usage error (exit 2, no
// report), distinct from the typed unsupported-platform refusal.
func TestCompiledInstalledEndpointUsageErrorStaysDistinct(t *testing.T) {
	t.Setenv("ARDENTS_DEBUG_SOCKET", "")
	command := exec.CommandContext(t.Context(), compiledCommand(t), "endpoint", "start-installed")
	var out, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &out, &diagnostic
	err := command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || out.Len() != 0 || diagnostic.Len() != 0 {
		t.Fatal("invalid endpoint shape lost the distinct usage-error refusal", err, out.String(), diagnostic.String())
	}
}

type unsupportedRefusalFailWriter struct{}

func (unsupportedRefusalFailWriter) Write([]byte) (int, error) {
	return 0, errors.New("refusal report write failed")
}

// The shared helper returns the refusal code 1 on a written report and 2 when
// the report itself cannot be written, matching the node-role stub grammar.
func TestUnsupportedPlatformRefusalExitCodes(t *testing.T) {
	var out bytes.Buffer
	if code := unsupportedPlatformRefusal(&out, "platform"); code != 1 {
		t.Fatalf("written refusal returned %d", code)
	}
	var event map[string]string
	if json.Unmarshal(out.Bytes(), &event) != nil || len(event) != 3 ||
		event["operation"] != "platform" || event["phase"] != "refused" || event["outcome"] != "unsupported-platform" {
		t.Fatal("shared helper lost the typed refusal grammar", out.String())
	}
	if code := unsupportedPlatformRefusal(unsupportedRefusalFailWriter{}, "platform"); code != 2 {
		t.Fatalf("unwritable refusal returned %d", code)
	}
}

func TestStreamCommandRefusesUnsupportedHostBeforeOpeningInputs(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "socket")
	link := filepath.Join(t.TempDir(), "link")
	for _, args := range [][]string{
		{"publish", socket},
		{"connect", socket, "--link-file", link},
	} {
		var diagnostic bytes.Buffer
		if code := runStreamCommand(t.Context(), args, &diagnostic); code != 1 ||
			diagnostic.String() != "Application stream failed: application stream requires the selected Linux host\n" {
			t.Fatalf("%v: exit %d, diagnostics %q", args, code, diagnostic.String())
		}
	}
	var diagnostic bytes.Buffer
	if code := runStreamCommand(t.Context(), []string{"connect", socket}, &diagnostic); code != 2 {
		t.Fatalf("invalid command exits %d: %s", code, diagnostic.String())
	}
}
