package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateResolutionTransportPackageIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	if _, err := os.Stat(filepath.Join(root, "internal", "naming", "resolution")); !os.IsNotExist(err) {
		t.Error("removed private-resolution transport directory still exists: internal/naming/resolution")
	}
	profile := string(readProjectFile(t, root, "tests/profiles/deterministic-packages.txt"))
	if strings.Contains(profile, "internal/naming/resolution") {
		t.Error("deterministic package profile still lists the removed private-resolution package")
	}
	allowlist := string(readProjectFile(t, root, "tests/profiles/deadcode-allowlist.json"))
	if strings.Contains(allowlist, "internal/naming/resolution.") {
		t.Error("deadcode allowlist still carries symbols of the removed private-resolution package")
	}
}

func TestResolutionRetirementPreservesRefusalAndBoundedFixture(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	command := string(readProjectFile(t, root, "cmd/ardents/name.go"))
	if !strings.Contains(command, `case "resolve", "control":`) ||
		!strings.Contains(command, "errNameNetworkCommandRetired") ||
		!strings.Contains(command, "name network command is retired; protected Service Name access is not selected") {
		t.Error("name command lost the exact retired resolve/control refusal")
	}

	fixture := string(readProjectFile(t, root, "cmd/ardents/name_retirement_fixture_test.go"))
	for _, forbidden := range []string{
		"nameresolution", "naming/resolution", "httptest", "GatewayProfile",
		"OpenResolutionGateway", "BindGatewayState",
	} {
		if strings.Contains(fixture, forbidden) {
			t.Errorf("bounded retirement fixture still composes removed transport: %q", forbidden)
		}
	}
	for _, retained := range []string{
		"prepareRetiredNameState(", "record.SignRecord(", "epoch.Open(",
		"CommitLegacy(", "admission.NewAdmission(", "retiredNameTreeUnchanged",
	} {
		if !strings.Contains(fixture, retained) {
			t.Errorf("bounded retirement fixture lost durable-root evidence %q", retained)
		}
	}

	oracle := string(readProjectFile(t, root, "cmd/ardents/name_retirement_test.go"))
	for _, retained := range []string{
		"TestNameNetworkCommandsRetireBeforeEffects",
		"name network command is retired; protected Service Name access is not selected",
		"retiredNameTreeUnchanged(t, fixture.stateRoot, stateBefore)",
		"retiredNameTreeUnchanged(t, fixture.namespaceRoot, namespaceBefore)",
	} {
		if !strings.Contains(oracle, retained) {
			t.Errorf("zero-effect retirement oracle lost %q", retained)
		}
	}

	view := string(readProjectFile(t, root, "internal/naming/namespace/resolution_view.go"))
	for _, retained := range []string{
		"func OpenResolutionGateway(", "func OpenResolutionVerifier(",
	} {
		if !strings.Contains(view, retained) {
			t.Errorf("Namespace resolution view lost %q pending the Namespace disposition", retained)
		}
	}
}
