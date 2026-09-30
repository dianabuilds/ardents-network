//go:build linux

package state_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// This verifies real State-to-participant composition up to the mandatory
// installed-worker refusal. It is not positive installed worker/read evidence.
func TestClosedReaderParticipantRetainsStateWithoutPublisher(t *testing.T) {
	for _, carrier := range []string{"ardents-carrier-tcp-tls-v2", "ardents-carrier-quic-v2"} {
		t.Run(carrier, func(t *testing.T) {
			testClosedIssuerProvisioningParticipant(t, carrier, 16, [32]byte{8}, nil,
				func(network state.Config, binary string, _ string, _ map[string]any) {
					root := t.TempDir()
					if err := os.Chmod(root, 0700); err != nil {
						t.Fatal(err)
					}
					path := func(name string) string { return filepath.Join(root, name) }
					if err := os.Mkdir(path("tokens"), 0700); err != nil {
						t.Fatal(err)
					}
					network.Now, network.Clock = time.Time{}, time.Now
					// This process fixture supplies live local clock observations. A
					// frozen observation expires during the three-second worker check.
					network.ClockObservation = time.Time{}
					network.ObserveClock = time.Now
					config := endpoint.ClosedParticipantConfig{ReaderOnly: true, Network: network,
						EntryRoot: path("entry"), LocalRoleRoot: path("roles"), TokenRoot: path("tokens"),
						ApplicationAddress: path("reader.sock"), BrokerID: [32]byte{91}, ConnectionPrincipal: [32]byte{92},
						ReaderPermission: endpoint.PermissionFiles{RequestPath: path("reader.request"), ResponsePath: path("reader.response"), Maxima: [3]uint32{1, 1, 1}},
						Observe: func(context.Context, endpoint.ClosedParticipantEvent) error {
							t.Error("unqualified participant emitted output")
							return nil
						},
					}
					ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
					defer cancel()
					if err := endpoint.RunClosedParticipant(ctx, config); err == nil || !strings.Contains(err.Error(), "text worker") {
						t.Fatalf("Reader did not reach the real worker boundary without an Instance: %v", err)
					} else {
						t.Logf("retained installed-worker refusal: %v", err)
					}
					for _, name := range []string{"instance", "publication", "publisher.request", "publisher.response", "admin.sock", "reader.sock", "reader.request", "reader.response"} {
						if _, err := os.Lstat(path(name)); !errors.Is(err, os.ErrNotExist) {
							t.Fatalf("Reader created %q: %v", name, err)
						}
					}
					// Reopening real owners proves joined cleanup released their leases.
					owner, err := state.Open(network)
					if err != nil {
						t.Fatal(err)
					}
					_, currentErr := owner.CurrentClosedRoute()
					if err := errors.Join(currentErr, owner.Close()); err != nil {
						t.Fatalf("Reader lost accepted State or its lease: %v", err)
					}
					// The real Linux command must carry the same selector into its
					// participant; parser-only acceptance cannot prove that transfer.
					clock := path("clock")
					t.Cleanup(startClockObserver(t, clock))
					public := hex.EncodeToString(network.ClosedProfileAuthority)
					plan := writeJSON(t, "reader-runtime.json", map[string]any{
						"schema": "ardents-headless-runtime-v2", "role": "reader", "network_state_root": network.Root,
						"entry_state_root": config.EntryRoot, "local_role_state_root": config.LocalRoleRoot, "text_token_root": config.TokenRoot,
						"application_socket": config.ApplicationAddress, "time_confidence_file": clock,
						"network_id": hex.EncodeToString(network.NetworkID[:]), "network_authorities": []string{public}, "network_threshold": 1,
						"network_profile": network.AcceptedProfile, "closed_profile_authority": public,
						"broker_id": identifierNode(91), "connection_principal": identifierNode(92),
						"reader_permission": map[string]any{"request_path": config.ReaderPermission.RequestPath, "response_path": config.ReaderPermission.ResponsePath, "maxima": [3]uint32{1, 1, 1}},
					})
					commandCtx, commandCancel := context.WithTimeout(t.Context(), 10*time.Second)
					defer commandCancel()
					output, commandErr := exec.CommandContext(commandCtx, binary, "endpoint", "headless", plan).CombinedOutput()
					if commandErr == nil || !bytes.Contains(output, []byte("text worker")) || bytes.Contains(output, []byte(`"kind":"ready"`)) {
						t.Fatalf("real Reader command did not preserve the worker refusal: %v / %s", commandErr, output)
					}
				})
		})
	}
}
