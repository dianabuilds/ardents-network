package node_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDomainCommandsUseSafeSharedObservationWithoutChangingReceipts(t *testing.T) {
	binary := buildNodeCommand(t)
	for _, tc := range []struct {
		operation string
		args      []string
		code      int
		outcome   string
	}{
		{"enrollment.artifact-name", []string{"enrollment", "artifact-name", "ardents", "linux-amd64"}, 0, "completed"},
		{"enrollment.verify", []string{"enrollment", "verify", "PRIVATE-input"}, 2, "usage-or-output-failure"},
		{"release.verify-initial", []string{"release", "verify-initial", "PRIVATE-input"}, 2, "usage-or-output-failure"},
		{"network.inspect", []string{"network", "inspect", "PRIVATE-input"}, 2, "usage-or-output-failure"},
		{"publication.prepare-root", []string{"publication", "prepare-root", "PRIVATE-input"}, 2, "usage-or-output-failure"},
		{"execution.prepare-permission", []string{"execution", "prepare-permission", "PRIVATE-input"}, 2, "usage-or-output-failure"},
	} {
		t.Run(tc.operation, func(t *testing.T) {
			command := exec.CommandContext(t.Context(), binary, tc.args...)
			var out, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &out, &diagnostic
			err := command.Run()
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			if code != tc.code {
				t.Fatalf("command outcome changed: code=%d err=%v", code, err)
			}
			var event map[string]string
			if err := json.Unmarshal(diagnostic.Bytes(), &event); err != nil || len(event) != 3 || event["operation"] != tc.operation || event["phase"] != "command" || event["outcome"] != tc.outcome {
				t.Fatalf("unclassified command result: %s (%v)", diagnostic.String(), err)
			}
			if bytes.Contains(diagnostic.Bytes(), []byte("PRIVATE")) {
				t.Fatal("argument escaped into labels")
			}
			projected := projectCommandTimeline(t, binary, diagnostic.Bytes())
			if !bytes.Contains(projected, []byte(tc.operation)) || !strings.Contains(string(projected), "\tcommand\t\""+tc.outcome+"\"") {
				t.Fatalf("command absent from timeline: %q", projected)
			}
			if tc.operation == "release.verify-initial" || tc.operation == "enrollment.verify" || tc.operation == "publication.prepare-root" {
				var receipt map[string]any
				if json.Unmarshal(out.Bytes(), &receipt) != nil || len(receipt) != 1 {
					t.Fatalf("stdout receipt changed: %s", out.String())
				}
			}
		})
	}
}

func TestEndpointCommandIdentityReachesSharedTimeline(t *testing.T) {
	binary := buildNodeCommand(t)
	root := filepath.Join(t.TempDir(), "PRIVATE-absent-installation")
	command := exec.CommandContext(t.Context(), binary, "endpoint", "start-installed", root)
	var out, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &out, &diagnostic
	var exit *exec.ExitError
	if err := command.Run(); !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatal("missing installation admitted", err)
	}
	var report map[string]string
	if json.Unmarshal(diagnostic.Bytes(), &report) != nil || report["operation"] != "endpoint.start-installed" {
		t.Fatalf("Endpoint identity missing: %s", diagnostic.String())
	}
	if out.Len() != 0 || strings.Contains(diagnostic.String(), "PRIVATE") {
		t.Fatal("private startup root entered observation")
	}
	projection := projectCommandTimeline(t, binary, diagnostic.Bytes())
	if !bytes.Contains(projection, []byte("endpoint.start-installed")) {
		t.Fatal("Endpoint refusal filtered")
	}
}

func TestApplicationCommandObservationExcludesPrivateArguments(t *testing.T) {
	binary := buildNodeCommand(t)
	for _, args := range [][]string{{"read", "private-sentinel-7c"}, {"stream", "connect", "private-sentinel-7c"}} {
		command := exec.CommandContext(t.Context(), binary, args...)
		var out, diagnostic bytes.Buffer
		command.Stdout, command.Stderr = &out, &diagnostic
		var exit *exec.ExitError
		if err := command.Run(); !errors.As(err, &exit) || exit.ExitCode() != 2 {
			t.Fatal("malformed Application input admitted", err)
		}
		count := 0
		for _, line := range bytes.Split(diagnostic.Bytes(), []byte("\n")) {
			var report map[string]string
			if json.Unmarshal(line, &report) != nil || report["operation"] == "" {
				continue
			}
			count++
			if len(report) != 3 || report["phase"] != "command" || report["outcome"] != "usage-or-output-failure" || !strings.HasPrefix(report["operation"], "application.") {
				t.Fatalf("invalid Application observation: %s", line)
			}
			if !bytes.Contains(projectCommandTimeline(t, binary, line), []byte(report["operation"])) {
				t.Fatal("Application result absent from shared reader")
			}
		}
		if count != 1 || bytes.Contains(diagnostic.Bytes(), []byte("private-sentinel-7c")) || out.Len() != 0 {
			t.Fatal("private Application input entered observation or result lost")
		}
	}
}
