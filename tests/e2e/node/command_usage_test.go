package node_test

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

func TestCommandUsageNamesSnapshotPublicationAndCommitBoundary(t *testing.T) {
	binary := buildNodeCommand(t)
	for _, arguments := range [][]string{nil, {"--help"}, {"help"}, {"unknown-command"}, {"inspect-permission", "private-permission", "private-facts"}} {
		cmd := exec.CommandContext(t.Context(), binary, arguments...)
		var out, diagnostic bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &diagnostic
		err := cmd.Run()
		explicitHelp := len(arguments) > 0 && (arguments[0] == "help" || arguments[0] == "--help")
		message := diagnostic.String()
		if explicitHelp {
			if err != nil || diagnostic.Len() != 0 {
				t.Fatalf("%v: error=%v stderr=%q", arguments, err, &diagnostic)
			}
			message = out.String()
		} else {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 || out.Len() != 0 {
				t.Fatalf("%v: error=%v stdout=%q", arguments, err, &out)
			}
		}
		for _, required := range []string{"publish <absolute-administration-socket> <absolute-document-file>", "Descriptor acknowledgement", "ardents endpoint withdraw", "Administration authorization"} {
			if !strings.Contains(message, required) {
				t.Errorf("help lacks %q: %s", required, message)
			}
		}
	}
}

func TestInfrastructureRejectsMissingAndUnknownOperation(t *testing.T) {
	binary := buildNodeCommand(t)
	for _, args := range [][]string{{"infrastructure"}, {"infrastructure", "prepare"}, {"infrastructure", "preflight-initial"}, {"infrastructure", "start-source", "/absent/distribution", "/absent/trust", "closed", "private", "/absent/binding"}} {
		cmd := exec.CommandContext(t.Context(), binary, args...)
		var out, diagnostic bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &diagnostic
		err := cmd.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || out.Len() != 0 {
			t.Errorf("%v: error=%v stdout=%q stderr=%q", args, err, &out, &diagnostic)
		}
	}
}

func TestPlatformTemplateIsAvailableWithoutNativeActivation(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), buildNodeCommand(t), "platform", "template")
	var out, diagnostic bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diagnostic
	if err := cmd.Run(); err != nil || diagnostic.Len() != 0 || out.String() != systemd.PlatformUnitTemplate {
		t.Fatalf("platform template: error=%v stderr=%q", err, &diagnostic)
	}
}

func TestBuiltHelpShowsTheSelectedRole(t *testing.T) {
	binary := buildNodeCommand(t)
	for _, test := range []struct {
		name string
		args []string
		want []string
		omit []string
	}{
		{"participant", []string{"--help"}, []string{"ardents stream publish", "ardents stream connect", "arbitrary bytes", "ardents publish", "ardents read", "ardents endpoint withdraw"}, []string{"ardents-custody", "serve-retained", "bounded UTF-8"}},
		{"operator", []string{"help", "operator"}, []string{"ardents enrollment verify", "ardents platform unit", "source serve-retained", "offline ardents-custody"}, []string{"ardents publish", "check-candidate"}},
		{"release", []string{"help", "release"}, []string{"ardents enrollment check-candidate", "<document-worker> <release-identity>", "ardents infrastructure sign-release", "separate release assets"}, []string{"ardents publish", "serve-retained"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, err := exec.CommandContext(t.Context(), binary, test.args...).Output()
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range test.want {
				if !strings.Contains(string(output), want) {
					t.Errorf("help lacks %q", want)
				}
			}
			for _, omit := range test.omit {
				if strings.Contains(string(output), omit) {
					t.Errorf("help unexpectedly includes %q", omit)
				}
			}
		})
	}
	result, err := exec.CommandContext(t.Context(), binary, "help", "unknown").Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || len(result) != 0 || !bytes.Contains(exit.Stderr, []byte("help [operator|release]")) {
		t.Fatalf("unknown help: stdout=%q error=%v", result, err)
	}
}
