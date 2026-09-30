package main

import (
	"errors"
	"strings"
	"testing"
)

func TestTextCommandHelpNamesSnapshotPublicationAndCommitBoundary(t *testing.T) {
	for _, arguments := range [][]string{nil, {"--help"}, {"help"}} {
		err := run(arguments)
		if !errors.Is(err, errTextUsage) {
			t.Fatalf("help returned %v", err)
		}
		message := textFailure(err)
		for _, required := range []string{"publish <absolute-administration-socket> <absolute-document-file>", "Descriptor acknowledgement", "ardents endpoint withdraw", "Administration authorization"} {
			if !strings.Contains(message, required) {
				t.Errorf("help lacks %q: %s", required, message)
			}
		}
	}
}

func TestCommandRejectsArbitraryEntrypointsAndArguments(t *testing.T) {
	for _, arguments := range [][]string{nil, {"shell"}, {"worker-reader", "destination"}, {"worker-publisher", "document-path"}} {
		if err := run(arguments); err == nil {
			t.Fatal("worker command accepted an unselected entrypoint or ambient input")
		}
	}
}
