package architecture

import (
	"strings"
	"testing"
)

// ADR-0102: the legacy Service Introduction key emission is retired. No
// production file may declare, persist, or echo introduction key material,
// and the X25519 exchange that generated it is gone from the Instance.
func TestLegacyIntroductionKeyEmissionIsRetired(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/service/instance/contract.go",
		"internal/service/instance/lifecycle.go",
		"internal/service/instance/request.go",
		"internal/service/instance/response.go",
		"internal/service/instance/root.go",
		"internal/service/instance/state.go",
		"internal/service/instance/storage.go",
		"internal/service/publication/contract.go",
		"internal/service/publication/credential.go",
		"internal/custody/service_credential.go",
		"internal/endpoint/service_runtime.go",
		"internal/endpoint/text_participant_linux.go",
	} {
		source := string(readProjectFile(t, root, relative))
		for _, banned := range []string{
			"IntroductionPublic",
			"IntroductionPrivate",
			"IntroductionHPKEPublic",
			"crypto/ecdh",
		} {
			if strings.Contains(source, banned) {
				t.Errorf("%s still references retired introduction material %q", relative, banned)
			}
		}
	}
}

// ADR-0102: the v3 grammars are the only emission path, and pre-v3 durable
// bytes meet typed ErrLegacyRoot refusals instead of a compatibility
// decoder or a phase-less rederivation branch.
func TestCredentialV3GrammarIsTheOnlyEmissionPath(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	checks := []struct {
		relative string
		required []string
	}{
		{"internal/service/publication/credential.go", []string{
			"credentialVersion  = uint16(3)",
			"credentialBodySize = 2 + 32 + 32 + 32 + 8 + 8 + 8 + 32 + 4",
		}},
		{"internal/service/instance/request.go", []string{
			"ardents-service-instance-request-v2",
			"const requestSize = len(requestDomain) + 32 + 32 + 8 + 8 + 32",
		}},
		{"internal/service/instance/response.go", []string{
			"ardents-service-instance-response-v2",
		}},
		{"internal/service/instance/root.go", []string{
			"ardents-service-instance-root-v2",
			"legacyMarker",
		}},
		{"internal/service/instance/state.go", []string{
			"ardents-service-instance-root-v2",
			"probe.Schema != stateSchema",
			"ErrLegacyRoot",
		}},
		{"internal/service/instance/storage.go", []string{
			"bytes.Equal(raw, []byte(legacyMarker))",
			"return ErrLegacyRoot",
		}},
		{"internal/service/instance/contract.go", []string{
			"ErrLegacyRoot = errors.New(",
			"re-initialize it",
		}},
		{"cmd/ardents/service_instance.go", []string{
			"ardents-service-instance-request-v2",
		}},
	}
	for _, check := range checks {
		source := string(readProjectFile(t, root, check.relative))
		for _, required := range check.required {
			if !strings.Contains(source, required) {
				t.Errorf("%s lost required ADR-0102 anchor %q", check.relative, required)
			}
		}
	}
	storage := string(readProjectFile(t, root, "internal/service/instance/storage.go"))
	if got := strings.Count(storage, "return ErrLegacyRoot"); got != 2 {
		t.Errorf("storage.go carries %d typed legacy refusals, want exactly 2 (prepareRoot + validateMarker)", got)
	}
	state := string(readProjectFile(t, root, "internal/service/instance/state.go"))
	if strings.Contains(state, "StatePending\n") && strings.Contains(state, "ecdh.X25519().GenerateKey") {
		t.Error("state.go still rederives introduction material")
	}
}
