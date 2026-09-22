package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredInitiatorReceivingClosureIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/entry/admission.go",
		"internal/entry/admission_test.go",
		"internal/route/credential/forward.go",
		"internal/route/credential/forward_redirect_test.go",
		"internal/service/reachability/gateway_redirect_test.go",
		"internal/service/reachability/gateway_tls_client.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Initiator Entry-admission file still exists: %s", relative)
		}
	}

	for _, check := range []struct {
		path      string
		forbidden []string
		retained  []string
	}{
		{
			path: "internal/route/entry_attachment.go",
			forbidden: []string{
				"type EntryAttachmentAcceptance struct",
				"func EntryAdmitterPort(",
				"func AcceptEntryAttachment(",
				"func readEntryBinding(",
			},
			retained: []string{"func OpenEntryAttachment("},
		},
		{
			path: "internal/route/entry_binding.go",
			forbidden: []string{
				"func DecodeEntryBinding(",
				"func AdmitEntryBinding(",
				"func ClientTLSPublicKey(",
			},
			retained: []string{"func EncodeEntryBinding(", "func ClientTLSKeyDigest("},
		},
		{
			path:      "internal/route/relay_setup.go",
			forbidden: []string{"func DecodeRelaySetup(", "func EncodeRelayReady("},
			retained:  []string{"func EncodeRelaySetup(", "func DecodeRelayReady("},
		},
		{
			path:      "internal/route/relay_setup_io.go",
			forbidden: []string{"func ReadRelaySetup(", "func WriteRelayReady("},
			retained:  []string{"func WriteRelaySetup(", "func ReadRelayReady("},
		},
		{
			path:      "internal/route/resolution_relay_setup.go",
			forbidden: []string{"func DecodeResolutionRelaySetup(", "func EncodeResolutionRelayReady("},
			retained:  []string{"func EncodeResolutionRelaySetup(", "func DecodeResolutionRelayReady("},
		},
		{
			path:      "internal/route/resolution_relay_envelope.go",
			forbidden: []string{"func DecodeResolutionRelayEnvelope(", "func EncodeResolutionRelayResponse("},
			retained:  []string{"func EncodeResolutionRelayEnvelope(", "func DecodeResolutionRelayResponse("},
		},
		{
			path: "internal/route/resolution_relay_io.go",
			forbidden: []string{
				"func ReadResolutionRelaySetup(",
				"func WriteResolutionRelayReady(",
				"func ReadResolutionRelayEnvelope(",
				"func WriteResolutionRelayResponse(",
			},
			retained: []string{
				"func WriteResolutionRelaySetup(",
				"func ReadResolutionRelayReady(",
				"func WriteResolutionRelayEnvelope(",
				"func ReadResolutionRelayResponse(",
			},
		},
		{
			path:      "internal/route/credential_relay_setup.go",
			forbidden: []string{"func DecodeCredentialRelaySetup(", "func EncodeCredentialRelayReady("},
			retained:  []string{"func EncodeCredentialRelaySetup(", "func DecodeCredentialRelayReady("},
		},
		{
			path: "internal/route/credential_relay_io.go",
			forbidden: []string{
				"func ReadCredentialRelaySetup(",
				"func WriteCredentialRelayReady(",
				"func ReadCredentialRelayEnvelope(",
				"func WriteCredentialRelayResponse(",
			},
			retained: []string{
				"func WriteCredentialRelaySetup(",
				"func ReadCredentialRelayReady(",
				"func WriteCredentialRelayEnvelope(",
				"func ReadCredentialRelayResponse(",
			},
		},
		{
			path:      "internal/service/reachability/private_relay.go",
			forbidden: []string{"func ForwardOHTTP("},
		},
	} {
		content := string(readProjectFile(t, root, check.path))
		for _, declaration := range check.forbidden {
			if strings.Contains(content, declaration) {
				t.Errorf("%s still contains retired declaration %q", check.path, declaration)
			}
		}
		for _, declaration := range check.retained {
			if !strings.Contains(content, declaration) {
				t.Errorf("%s lost retained User Route declaration %q", check.path, declaration)
			}
		}
	}
}
