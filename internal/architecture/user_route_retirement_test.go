package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredUserRouteOwnerIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/route/route.go",
		"internal/route/user_attachment.go",
		"internal/route/user_attachment_test.go",
		"internal/route/user_private_reachability.go",
		"internal/route/user_introduction.go",
		"internal/route/user_introduction_test.go",
		"internal/route/relay_setup.go",
		"internal/route/relay_setup_io.go",
		"internal/route/relay_setup_test.go",
		"internal/route/resolution_relay_envelope.go",
		"internal/route/resolution_relay_io.go",
		"internal/route/resolution_relay_setup.go",
		"internal/route/resolution_relay_test.go",
		"internal/endpoint/user_route_credential_integration_test.go",
		"internal/endpoint/user_route_credential_introduction_fixture_test.go",
		"internal/endpoint/user_route_initiator_fixture_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired User Route owner file still exists: %s", relative)
		}
	}

	forbidden := []string{
		"type StateView interface",
		"type UserEntry interface",
		"type ResourceAdmission func",
		"type TransitPeer struct",
		"type CredentialExchange func",
		"type CredentialRequest struct",
		"type Credential struct",
		"type CredentialAcquirer func",
		"type Config struct",
		"type Intent struct",
		"type Route struct",
		"func Open(input Config) (*Route",
		"func (route *Route)",
	}
	entries, err := os.ReadDir(filepath.Join(root, "internal", "route"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(root, "internal", "route", entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range forbidden {
			if strings.Contains(string(content), declaration) {
				t.Errorf("%s still contains retired User Route declaration %q", entry.Name(), declaration)
			}
		}
	}

	for _, check := range []struct {
		path         string
		declarations []string
	}{
		{
			path: "internal/route/native_attachment.go",
			declarations: []string{
				"type Attachment struct",
				"type Evidence struct",
			},
		},
	} {
		content := string(readProjectFile(t, root, check.path))
		for _, declaration := range check.declarations {
			if !strings.Contains(content, declaration) {
				t.Errorf("%s lost retained declaration %q", check.path, declaration)
			}
		}
	}

	controlIO := string(readProjectFile(t, root, "internal/route/introduction_control_io.go"))
	if strings.Contains(controlIO, "func WriteSealedIntroduction(") {
		t.Error("Introduction control IO still contains the retired User sealed-submission writer")
	}
	for _, retained := range []string{"func WriteIntroductionSlotRegistration(", "func ReadIntroductionControlRecord("} {
		if !strings.Contains(controlIO, retained) {
			t.Errorf("Introduction control IO lost retained declaration %q", retained)
		}
	}

	staleCompletion := string(readProjectFile(t, root, "internal/endpoint/transit/stale_completion_test.go"))
	if strings.Contains(staleCompletion, "func TestPublisherAndUserShareOneIntroductionCompletionOwner(") {
		t.Error("Transit stale-completion tests still contain the retired User Route composition lane")
	}
	if !strings.Contains(staleCompletion, "func TestTransitCredentialLifecycleIgnoresStaleIssuerOutcomes(") {
		t.Error("Transit lost the retained transit-acquisition stale-outcome oracle")
	}
}
