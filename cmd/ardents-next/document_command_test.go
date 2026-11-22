package main

import (
	"errors"
	"strings"
	"testing"
)

func TestDocumentCommandHelpNamesSnapshotPublicationAndCommitBoundary(t *testing.T) {
	for _, arguments := range [][]string{nil, {"--help"}, {"help"}} {
		err := executeDocumentCommand(t.Context(), arguments)
		if !errors.Is(err, errDocumentUsage) {
			t.Fatalf("help returned %v", err)
		}
		message := documentFailure(err)
		for _, required := range []string{"publish <absolute-administration-socket> <absolute-document-file>", "Descriptor acknowledgement", "ardents endpoint withdraw", "Administration authorization"} {
			if !strings.Contains(message, required) {
				t.Errorf("help lacks %q: %s", required, message)
			}
		}
	}
}
