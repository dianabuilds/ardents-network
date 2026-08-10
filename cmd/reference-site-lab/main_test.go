package main

import "testing"

func TestRunRejectsMissingUnknownAndMalformedCommands(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		arguments []string
		status    int
	}{
		{name: "missing command", status: 2},
		{name: "unknown command", arguments: []string{"unknown"}, status: 2},
		{name: "unknown role flag", arguments: []string{"role", "--unknown"}, status: 2},
		{name: "probe trailing argument", arguments: []string{"probe", "trailing"}, status: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			if status := run(test.arguments); status != test.status {
				t.Fatalf("run(%q) = %d, want %d", test.arguments, status, test.status)
			}
		})
	}
}

func TestRunDispatchesKnownRoleAndProbeFailures(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{
		{"role", "--role", "unknown"},
		{"probe", "--kind", "application"},
	} {
		if status := run(arguments); status != 1 {
			t.Errorf("run(%q) = %d, want 1", arguments, status)
		}
	}
}
