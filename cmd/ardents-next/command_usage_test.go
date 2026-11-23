package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestCommandUsageNamesSnapshotPublicationAndCommitBoundary(t *testing.T) {
	for _, arguments := range [][]string{nil, {"--help"}, {"help"}, {"unknown-command"}} {
		var out, diagnostic bytes.Buffer
		code := run(t.Context(), arguments, &out, &diagnostic)
		explicitHelp := len(arguments) > 0 && (arguments[0] == "help" || arguments[0] == "--help")
		message := diagnostic.String()
		if explicitHelp {
			if code != 0 || diagnostic.Len() != 0 {
				t.Fatalf("%v: code=%d stderr=%q", arguments, code, &diagnostic)
			}
			message = out.String()
		} else if code != 2 || out.Len() != 0 {
			t.Fatalf("%v: code=%d stdout=%q", arguments, code, &out)
		}
		for _, required := range []string{"publish <absolute-administration-socket> <absolute-document-file>", "Descriptor acknowledgement", "ardents endpoint withdraw", "Administration authorization"} {
			if !strings.Contains(message, required) {
				t.Errorf("help lacks %q: %s", required, message)
			}
		}
	}
}

func TestInfrastructureRejectsMissingAndUnknownOperation(t *testing.T) {
	for _, args := range [][]string{{"infrastructure"}, {"infrastructure", "prepare"}, {"infrastructure", "preflight-initial"}} {
		var out, diagnostic bytes.Buffer
		if code := run(t.Context(), args, &out, &diagnostic); code != 2 || out.Len() != 0 {
			t.Errorf("%v: code=%d stdout=%q stderr=%q", args, code, &out, &diagnostic)
		}
	}
}

func TestPlatformTemplateIsAvailableWithoutNativeActivation(t *testing.T) {
	var out, diagnostic bytes.Buffer
	code := run(t.Context(), []string{"platform", "template"}, &out, &diagnostic)
	if code != 0 || diagnostic.Len() != 0 || out.String() != platformUnitTemplate {
		t.Fatalf("platform template: code=%d, stderr=%q", code, diagnostic.String())
	}
}

func TestBuiltHelpShowsTheSelectedRole(t *testing.T) {
	binary := compiledCommand(t)
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
