package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR-0100 removed the uncomposed private-resolution transport. ADR-0105
// then retired the whole Namespace control subsystem: the PO confirmed that
// no deployed Namespace root deserves data support, so typed incompatibility
// is the absence of any read path. Old roots stay on disk byte-for-byte;
// nothing in the working tree can open, convert, or delete them.
func TestPrivateResolutionTransportPackageIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		filepath.Join("internal", "naming", "resolution"),
		filepath.Join("internal", "naming", "namespace"),
	} {
		if _, err := os.Stat(filepath.Join(root, relative)); !os.IsNotExist(err) {
			t.Errorf("removed naming subsystem directory still exists: %s", relative)
		}
	}
	profile := string(readProjectFile(t, root, "tests/profiles/deterministic-packages.txt"))
	allowlist := string(readProjectFile(t, root, "tests/profiles/deadcode-allowlist.json"))
	for _, forbidden := range []string{"internal/naming/resolution", "internal/naming/namespace"} {
		if strings.Contains(profile, forbidden) {
			t.Errorf("deterministic package profile still lists the removed package %q", forbidden)
		}
		if strings.Contains(allowlist, forbidden+".") {
			t.Errorf("deadcode allowlist still carries symbols of the removed package %q", forbidden)
		}
	}
}

// ADR-0105 also retired the unexposed custody Namespace operations and the
// uncalled generation-2 reachability writers, per the deadcode registry's own
// superseding-decision rule for the reachability tracer group.
func TestNamespaceRetirementRemovesUnexposedWriters(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/custody/vault_namespace_preparation.go",
		"internal/custody/vault_reconciliation.go",
		"internal/custody/vault_namespace_signing_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired custody Namespace file still exists: %s", relative)
		}
	}
	contract := string(readProjectFile(t, root, "internal/custody/vault_contract.go"))
	for _, forbidden := range []string{
		"OperationSignNamespaceTransition", "OperationPrepareNamespaceSubmission",
		"OperationActivateRecoveredAuthority", "NamespaceTransition", "NamespaceSubmission",
	} {
		if strings.Contains(contract, forbidden) {
			t.Errorf("custody contract still declares retired Namespace member %q", forbidden)
		}
	}
	// ADR-0109 (F-32) deleted the retained generation-2 decode grammar file
	// outright; the typed refusal lives in the stored-record envelope instead.
	if _, err := os.Stat(filepath.Join(root, "internal", "service", "reachability", "descriptor.go")); !os.IsNotExist(err) {
		t.Error("retired reachability descriptor grammar file still exists")
	}
	store := string(readProjectFile(t, root, "internal/service/reachability/store.go"))
	reachabilityContract := string(readProjectFile(t, root, "internal/service/reachability/contract.go"))
	for _, retired := range []struct{ source, name string }{
		{store, "func (store *Store) Publish("},
		{store, "func (store *Store) Lookup("},
		{reachabilityContract, "IssueInput"},
		{reachabilityContract, "SubmissionMode"},
		{reachabilityContract, "type Introduction struct"},
	} {
		if strings.Contains(retired.source, retired.name) {
			t.Errorf("reachability still declares retired generation-2 surface %q", retired.name)
		}
	}
	for _, forbidden := range []string{"func encodeBody(", "func verifyStored(", "func Verify(", "func decode("} {
		if strings.Contains(store, forbidden) {
			t.Errorf("reachability still declares retired generation-2 helper %q", forbidden)
		}
	}
	if !strings.Contains(store, "func (store *Store) lookup(") ||
		!strings.Contains(store, "func compareStored(") {
		t.Error("reachability lost the retained private floor comparison")
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
		"nameresolution", "naming/resolution", "naming/namespace", "httptest", "GatewayProfile",
		"OpenResolutionGateway", "BindGatewayState", "epoch.Open(", "record.SignRecord(",
		"CommitLegacy(", "admission.NewAdmission(",
	} {
		if strings.Contains(fixture, forbidden) {
			t.Errorf("bounded retirement fixture still composes removed machinery: %q", forbidden)
		}
	}
	for _, retained := range []string{
		"prepareRetiredNameState(", "retiredNameSyntheticNamespaceRoot(", "retiredNameTreeUnchanged",
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
}
