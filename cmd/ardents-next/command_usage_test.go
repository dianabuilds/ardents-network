package main

import (
	"bytes"
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
