//go:build linux

package issuer

import (
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClosedTokenIssuerRetainedLease(t *testing.T) {
	for _, scenario := range []string{"second_owner", "existing_owner", "reopen"} {
		t.Run(scenario, func(t *testing.T) {
			issuer, profile, operation, now, _ := closedTokenListenerIssuer(t)
			defer issuer.Close()
			cfg := ClosedTokenIssuerConfig{Root: issuer.root, NetworkID: issuer.network, CurrentProfile: func() (state.ClosedProfileView, bool) { return profile, true }, Clock: func() time.Time { return now }}
			first, err := issuer.IssueTerminalOperation(operation)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := terminal.DecodeIssuanceResult(first, [32]byte{71})
			if err != nil {
				t.Fatal(err)
			}
			batch, err := admissiontoken.DecodeClosedTokenBatchResult(decoded.Payload)
			if err != nil || batch.Status != admissiontoken.ClosedTokenIssued {
				t.Fatalf("healthy issue: %v / %v", batch.Status, err)
			}
			if scenario == "reopen" {
				if err := issuer.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Remove(filepath.Join(cfg.Root, issuerRootLockName)); err != nil {
				t.Fatal(err)
			}
			if scenario != "existing_owner" {
				other, err := OpenClosedTokenIssuer(cfg)
				if err == nil {
					other.Close()
					t.Fatal("retained root with missing lease admitted another issuer owner")
				}
			} else {
				result, err := issuer.IssueTerminalOperation(operation)
				if err != nil {
					return
				}
				decoded, err := terminal.DecodeIssuanceResult(result, [32]byte{71})
				if err != nil {
					return
				}
				batch, err := admissiontoken.DecodeClosedTokenBatchResult(decoded.Payload)
				if err == nil && batch.Status == admissiontoken.ClosedTokenIssued {
					t.Fatal("issuer returned a signed result after loss of its exclusive lease path")
				}
			}
		})
	}
}

func TestClosedTokenIssuerLeaseLossDoesNotRevive(t *testing.T) {
	for _, change := range []string{"lock", "root"} {
		t.Run(change, func(t *testing.T) {
			issuer, _, operation, _, _ := closedTokenListenerIssuer(t)
			defer issuer.Close()
			status := func() admissiontoken.ClosedTokenBatchStatus {
				t.Helper()
				raw, err := issuer.IssueTerminalOperation(operation)
				if err != nil {
					t.Fatal(err)
				}
				result, err := terminal.DecodeIssuanceResult(raw, [32]byte{71})
				if err != nil {
					t.Fatal(err)
				}
				batch, err := admissiontoken.DecodeClosedTokenBatchResult(result.Payload)
				if err != nil {
					t.Fatal(err)
				}
				return batch.Status
			}
			if status() != admissiontoken.ClosedTokenIssued {
				t.Fatal("healthy issuance refused")
			}
			path := filepath.Join(issuer.root, issuerRootLockName)
			if change == "root" {
				path = issuer.root
			}
			saved := filepath.Join(t.TempDir(), "retained")
			if err := os.Rename(path, saved); err != nil {
				t.Fatal(err)
			}
			if change == "root" {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if status() != admissiontoken.ClosedTokenUnavailable {
				t.Fatal("changed identity still issued")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(saved, path); err != nil {
				t.Fatal(err)
			}
			if status() != admissiontoken.ClosedTokenUnavailable {
				t.Fatal("restoring path revived failed owner")
			}
			first := issuer.Close()
			if first == nil || issuer.Close() != first {
				t.Fatal("Close did not retain lost ownership failure")
			}
		})
	}
}
