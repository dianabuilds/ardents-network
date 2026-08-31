//go:build h4_2_exact_candidate || referencec2

package service_test

import (
	"os"
	"strings"
	"testing"
)

func buildE2EFixtureCommand(t *testing.T, name string) string {
	t.Helper()
	prebuilt := os.Getenv("ARDENTS_E2E_FIXTURE_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
	if prebuilt != "" {
		info, err := os.Stat(prebuilt)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("prebuilt e2e fixture %q is not a regular file: %v", name, err)
		}
		return prebuilt
	}
	return buildCommand(t, name, "./tests/e2e/service/fixturecommand/"+name)
}
