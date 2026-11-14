//go:build linux

package issuer

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

func TestLegacyIssuerPublicReopenPreservesKeysResultsAndQuota(t *testing.T) {
	original, profile, firstOperation, now, verifyFirst := closedTokenListenerIssuer(t)
	t.Cleanup(func() { _ = original.Close() })
	firstResult, err := original.IssueTerminalOperation(firstOperation)
	if err != nil {
		t.Fatal(err)
	}
	verifyFirst(firstResult)
	requestFrame, err := terminal.DecodeIssuanceRequest(firstOperation)
	if err != nil {
		t.Fatal(err)
	}
	requestRaw, err := admissiontoken.DecodePaddedBatch(requestFrame.Payload)
	if err != nil {
		t.Fatal(err)
	}
	firstRequest, err := admissiontoken.DecodeClosedTokenBatch(requestRaw)
	if err != nil {
		t.Fatal(err)
	}
	prepare := func() []byte {
		t.Helper()
		context := admissiontoken.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest,
			ReceiverNodeID: sha256.Sum256([]byte("listener receiver")), IssuerNodeID: profile.IssuerNodeID,
			ReceiverDutyGeneration: 6, Class: 1, WindowStart: firstRequest.WindowStart}
		pending, err := admissiontoken.PrepareClosedTokenBatch(admissiontoken.ClosedTokenBatchConfig{Profile: profile,
			Contexts: []admissiontoken.ClosedTokenContext{context}, Permission: firstRequest.Permission,
			HolderKey: ed25519.NewKeyFromSeed(bytesForClosedTokenBatch(73)), Now: now})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pending.Discard)
		operation, err := terminal.EncodeIssuanceRequest([32]byte{71}, pending.Request())
		if err != nil {
			t.Fatal(err)
		}
		return operation
	}
	secondOperation, thirdOperation := prepare(), prepare()
	secondResult, err := original.IssueTerminalOperation(secondOperation)
	if err != nil {
		t.Fatal(err)
	}
	secondFrame, err := terminal.DecodeIssuanceResult(secondResult, [32]byte{71})
	if err != nil || secondFrame.Status != 0 {
		t.Fatalf("second committed request: status=%d err=%v", secondFrame.Status, err)
	}
	if err := original.Close(); err != nil {
		t.Fatal(err)
	}
	// Reconstruct the historical grammar from real committed requests, without
	// the ledger's header/record encoder or slicing its production output.
	legacy, promoted := []byte("ARDILG01"), []byte("ARDILG02")
	for _, identity := range [][32]byte{profile.NetworkID, profile.IssuerNodeID, profile.Digest} {
		legacy = append(legacy, identity[:]...)
		promoted = append(promoted, identity[:]...)
	}
	for _, operation := range [][]byte{firstOperation, secondOperation} {
		frame, err := terminal.DecodeIssuanceRequest(operation)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := admissiontoken.DecodePaddedBatch(frame.Payload)
		if err != nil {
			t.Fatal(err)
		}
		request, err := admissiontoken.DecodeClosedTokenBatch(raw)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		record := make([]byte, 108)
		copy(record[0:32], request.RequestID[:])
		copy(record[32:64], digest[:])
		copy(record[64:96], request.Permission.PermissionID[:])
		binary.BigEndian.PutUint64(record[96:104], uint64(request.WindowStart.Unix()))
		record[104] = request.Class
		binary.BigEndian.PutUint16(record[105:107], uint16(len(request.BlindedRequests)))
		record[107] = 1
		legacy = append(legacy, record...)
		promoted = append(promoted, record[:105]...)
		promoted = append(promoted, 1)
		promoted = append(promoted, record[105:]...)
	}
	template := legacyRootImage(t, original.root)
	for _, scenario := range []string{"legacy-unbound", "legacy-bound", "empty-stage", "partial-stage", "complete-stage",
		"current-complete", "current-leftover-stage", "partial-binding", "conflicting-binding", "missing-binding",
		"missing-journal", "malformed-record", "nonregular-stage"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0o700); err != nil {
				t.Fatal(err)
			}
			write := func(name string, raw []byte) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, name), raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for name, value := range template {
				write(name, []byte(value))
			}
			write("closed-token-ledger", legacy)
			write(".closed-token-ledger-binding", promoted[:104])
			remove := func(name string) {
				t.Helper()
				if err := os.Remove(filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
			}
			refuse := false
			switch scenario {
			case "legacy-unbound":
				remove(".closed-token-ledger-binding")
			case "empty-stage":
				write(".closed-token-ledger-next", nil)
			case "partial-stage":
				write(".closed-token-ledger-next", promoted[:140])
			case "complete-stage":
				write(".closed-token-ledger-next", promoted)
			case "current-complete", "current-leftover-stage":
				write("closed-token-ledger", promoted)
				if scenario == "current-leftover-stage" {
					write(".closed-token-ledger-next", []byte("uncommitted residue"))
				}
			case "partial-binding", "conflicting-binding":
				refuse = true
				binding := append([]byte(nil), promoted[:104]...)
				if scenario == "partial-binding" {
					binding = binding[:52]
				} else {
					binding[8] ^= 1
				}
				write(".closed-token-ledger-binding", binding)
			case "missing-binding":
				refuse = true
				write("closed-token-ledger", promoted)
				remove(".closed-token-ledger-binding")
			case "missing-journal":
				refuse = true
				remove("closed-token-ledger")
			case "malformed-record":
				refuse = true
				broken := append([]byte(nil), legacy...)
				broken[211] = 2
				write("closed-token-ledger", broken)
			case "nonregular-stage":
				refuse = true
				if err := os.Mkdir(filepath.Join(root, ".closed-token-ledger-next"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			before := legacyRootImage(t, root)
			keyBefore, err := os.Stat(filepath.Join(root, "closed-issuer-keys"))
			if err != nil {
				t.Fatal(err)
			}
			cfg := ClosedTokenIssuerConfig{Root: root, NetworkID: profile.NetworkID,
				CurrentProfile: func() (state.ClosedProfileView, bool) { return profile, true }, Clock: func() time.Time { return now }}
			for range 2 {
				owner, err := OpenClosedTokenIssuer(cfg)
				if refuse {
					if err == nil || owner != nil {
						if owner != nil {
							_ = owner.Close()
						}
						t.Fatal("damaged retained history accepted")
					}
					if !reflect.DeepEqual(before, legacyRootImage(t, root)) {
						t.Fatal("refusal changed retained history")
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = owner.Close() })
				other, err := OpenClosedTokenIssuer(cfg)
				if err == nil || other != nil {
					if other != nil {
						_ = other.Close()
					}
					t.Fatal("recovery admitted competing owner")
				}
				for index, operation := range [][]byte{firstOperation, secondOperation} {
					got, err := owner.IssueTerminalOperation(operation)
					want := [][]byte{firstResult, secondResult}[index]
					if err != nil || !bytes.Equal(got, want) {
						t.Fatalf("retained exact response changed: %v", err)
					}
				}
				got, err := owner.IssueTerminalOperation(thirdOperation)
				if err != nil {
					t.Fatal(err)
				}
				frame, err := terminal.DecodeIssuanceResult(got, [32]byte{71})
				if err != nil || frame.Status != 2 {
					t.Fatalf("migration refunded spent permission: status=%d err=%v", frame.Status, err)
				}
				if err := owner.Close(); err != nil {
					t.Fatal(err)
				}
				image := legacyRootImage(t, root)
				if !bytes.Equal([]byte(image["closed-token-ledger"]), promoted) || image["closed-issuer-keys"] != before["closed-issuer-keys"] {
					t.Fatal("reopen rewrote committed history or private material")
				}
				if _, exists := image[".closed-token-ledger-next"]; exists {
					t.Fatal("recovery left staging residue")
				}
				keyAfter, err := os.Stat(filepath.Join(root, "closed-issuer-keys"))
				if err != nil || !os.SameFile(keyBefore, keyAfter) || !keyBefore.ModTime().Equal(keyAfter.ModTime()) {
					t.Fatal("reopen replaced private material")
				}
			}
		})
	}
}

// Synthetic fixture bytes stay in the test process; failures never print them.
func legacyRootImage(t *testing.T, root string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	image := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			image[entry.Name()+"/"] = "directory"
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		image[entry.Name()] = string(raw)
	}
	return image
}
