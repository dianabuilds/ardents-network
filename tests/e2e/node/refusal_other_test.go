//go:build !linux

package node_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledNodeUnsupportedPlatformRefusesBeforePrivateInput(t *testing.T) {
	t.Setenv("ARDENTS_DEBUG_SOCKET", "")
	binary := buildNodeCommand(t)
	var routes [][]string
	for _, role := range []string{"forwarding", "issuer", "descriptor", "introduction", "join"} {
		for _, flag := range []string{"--config", "--retained-config"} {
			routes = append(routes, []string{"node", role, flag})
		}
		if role != "issuer" {
			routes = append(routes, []string{"node", role, "repair-retained"})
		}
	}
	for _, route := range routes {
		t.Run(strings.Join(route[1:], "/"), func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "unavailable-private-input")
			args := append(route, input)
			command := exec.CommandContext(t.Context(), binary, args...)
			var out, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &out, &diagnostic
			err := command.Run()
			var exit *exec.ExitError
			want := `{"operation":"node.` + route[1] + `","phase":"refused","outcome":"unsupported-platform"}` + "\n"
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || out.String() != want {
				t.Fatal("compiled Node read unavailable private input or lost native refusal", err, out.String())
			}
			projection := projectCommandTimeline(t, binary, out.Bytes())
			wantTimeline := "-\tunavailable\tnode\t-\t-\t-\tnode." + route[1] + "\trefused\t\"unsupported-platform\"\n"
			if string(projection) != wantTimeline {
				t.Fatalf("current Node report disappeared from timeline: %q", projection)
			}
			if diagnostic.Len() != 0 {
				t.Fatal("unsupported Node exposed private input or a native cause")
			}
			if _, err := os.Stat(input); !os.IsNotExist(err) {
				t.Fatal("unsupported Node created private runtime input", err)
			}
		})
	}
}

func TestHostingUnsupportedDoesNotReadInput(t *testing.T) {
	input := filepath.Join(t.TempDir(), "missing")
	command := exec.CommandContext(t.Context(), buildNodeCommand(t), "hosting", "observe", "--config", input)
	var output, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &output, &diagnostic
	err := command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 ||
		!bytes.Contains(output.Bytes(), []byte("unsupported-platform")) || diagnostic.Len() != 0 {
		t.Fatalf("hosting refusal: %v, stdout %q, stderr %q", err, &output, &diagnostic)
	}
	if _, err := os.Stat(input); !os.IsNotExist(err) {
		t.Fatal("unsupported command touched input", err)
	}
}

func TestStreamCommandRefusesUnsupportedHostBeforeOpeningInputs(t *testing.T) {
	binary := buildNodeCommand(t)
	socket := filepath.Join(t.TempDir(), "socket")
	link := filepath.Join(t.TempDir(), "link")
	for _, args := range [][]string{
		{"stream", "publish", socket},
		{"stream", "connect", socket, "--link-file", link},
	} {
		command := exec.CommandContext(t.Context(), binary, args...)
		var output, diagnostic bytes.Buffer
		command.Stdout, command.Stderr = &output, &diagnostic
		err := command.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 || output.Len() != 0 ||
			diagnostic.String() != "Application stream failed: application stream requires the selected Linux host\n" {
			t.Fatalf("%v: error %v, stdout %q, stderr %q", args, err, &output, &diagnostic)
		}
	}
	command := exec.CommandContext(t.Context(), binary, "stream", "connect", socket)
	var output bytes.Buffer
	command.Stdout = &output
	err := command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || output.Len() != 0 {
		t.Fatalf("invalid command: error %v, stdout %q", err, &output)
	}
	for _, path := range []string{socket, link} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("unsupported command touched %s: %v", path, err)
		}
	}
}

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
		{"route", []string{"route", "receive"}, "route", false},
		{"endpoint-start-installed", []string{"endpoint", "start-installed", "/nonexistent-socket"}, "endpoint", true},
		{"endpoint-prepare-history", []string{"endpoint", "prepare-history", "a", "b", "c"}, "endpoint", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			command := exec.CommandContext(t.Context(), buildNodeCommand(t), tc.args...)
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
	command := exec.CommandContext(t.Context(), buildNodeCommand(t), "endpoint", "start-installed")
	var out, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &out, &diagnostic
	err := command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || out.Len() != 0 || diagnostic.Len() != 0 {
		t.Fatal("invalid endpoint shape lost the distinct usage-error refusal", err, out.String(), diagnostic.String())
	}
}

// A failed report remains a usage-style failure and does not open private inputs.
func TestNodeUnsupportedPlatformReportsWriteFailure(t *testing.T) {
	binary := buildNodeCommand(t)
	for _, route := range [][]string{
		{"node", "forwarding", "--config"}, {"node", "issuer", "--config"},
		{"node", "descriptor", "--config"}, {"node", "introduction", "--config"}, {"node", "join", "--config"},
		{"node", "forwarding", "repair-retained"}, {"node", "descriptor", "repair-retained"},
		{"node", "introduction", "repair-retained"}, {"node", "join", "repair-retained"},
	} {
		t.Run(strings.Join(route[1:], "/"), func(t *testing.T) {
			dir := t.TempDir()
			report := filepath.Join(dir, "readonly")
			if err := os.WriteFile(report, nil, 0600); err != nil {
				t.Fatal(err)
			}
			output, err := os.Open(report)
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			input := filepath.Join(dir, "unavailable-private-input")
			command := exec.CommandContext(t.Context(), binary, append(route, input)...)
			command.Stdout = output
			var diagnostic bytes.Buffer
			command.Stderr = &diagnostic
			err = command.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 || diagnostic.Len() != 0 {
				t.Fatalf("failed refusal: %v stderr %q", err, &diagnostic)
			}
			if _, err := os.Lstat(input); !os.IsNotExist(err) {
				t.Fatalf("private input touched: %v", err)
			}
		})
	}
}
