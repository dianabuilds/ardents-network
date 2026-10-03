//go:build linux

package custody

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
)

type advancingIssuanceSecret struct {
	password []byte
	advance  func()
}

func (s advancingIssuanceSecret) ReadSecret(context.Context, SecretPrompt) ([]byte, error) {
	s.advance()
	return append([]byte(nil), s.password...), nil
}
func (s advancingIssuanceSecret) Confirm(context.Context, ConfirmationPrompt) (bool, error) {
	return false, nil
}

func TestIssuanceRefusesTimeChangeBeforeDurableCommit(t *testing.T) {
	for _, kind := range []AuthorityKind{AuthorityService, AuthorityAdmission} {
		stages := []string{"unlock", "seal", "valid"}
		sealRead := 3
		if kind == AuthorityAdmission {
			// Admission also rechecks the hour after the encrypted record is opened.
			stages = []string{"unlock", "open", "seal", "valid"}
			sealRead = 4
		}
		for _, stage := range stages {
			t.Run(string(kind)+"/"+stage, func(t *testing.T) {
				now := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
				clock := now
				calls := 0
				armed := false
				vault, err := Open(VaultConfig{Root: t.TempDir(), Now: func() time.Time {
					if armed {
						calls++
						if stage == "seal" && calls == sealRead || stage == "open" && calls == 3 {
							clock = now.Add(time.Hour)
						}
					}
					return clock
				}})
				if err != nil {
					t.Fatal(err)
				}
				defer vault.Close()
				binding := AuthorityBinding{Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}, Kind: kind}
				password := []byte("issuance time boundary password")
				creation := OperationCreateServiceAuthority
				if kind == AuthorityAdmission {
					creation = OperationCreateAdmissionAuthority
				}
				made, err := vault.Execute(t.Context(), Operation{Kind: creation, Authority: AuthorityState{Binding: binding}}, &sequenceSecrets{values: [][]byte{password, password}})
				if err != nil {
					t.Fatal(err)
				}
				operation := Operation{RecordID: made.RecordID, Expected: made.Authority.Binding}
				if kind == AuthorityService {
					raw := serviceInstanceRequest(t, binding.Network, now, now.Add(time.Minute))
					operation.Kind = OperationIssueServiceCredential
					operation.ServiceRequest = raw
					operation.ServiceRequestCommitment = sha256.Sum256(raw)
				} else {
					request, holder, err := admission.PreparePermissionRequest(made.AdmissionAuthority.Public, binding.Network, [32]byte{4}, 5, admission.AllocationUser, now, [3]uint32{1, 0, 0})
					if err != nil {
						t.Fatal(err)
					}
					defer zero(holder)
					raw, err := admission.EncodePermissionRequest(request)
					if err != nil {
						t.Fatal(err)
					}
					operation.Kind = OperationIssueAdmissionPermission
					operation.AdmissionRequest = raw
					operation.AdmissionRequestCommitment = sha256.Sum256(raw)
				}
				before, err := vault.readFloors()
				if err != nil {
					t.Fatal(err)
				}
				records, err := os.ReadDir(vault.records)
				if err != nil {
					t.Fatal(err)
				}
				armed = true
				receipt, err := vault.Execute(t.Context(), operation, advancingIssuanceSecret{password: password, advance: func() {
					if stage == "unlock" {
						clock = now.Add(time.Hour)
					}
				}})
				expectedReads := sealRead
				if stage == "unlock" {
					expectedReads = 2
				}
				if stage == "open" {
					expectedReads = 3
				}
				if calls != expectedReads {
					t.Fatalf("scenario did not reach its intended time boundary: calls=%d want=%d", calls, expectedReads)
				}
				if stage == "valid" {
					if err != nil || receipt.State != RecordActive {
						t.Fatalf("valid issuance: %+v / %v", receipt, err)
					}
					return
				}
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("elapsed issuance: %+v / %v", receipt, err)
				}
				after, err := vault.readFloors()
				if err != nil {
					t.Fatal(err)
				}
				if !equalFloors(before, after) {
					t.Fatal("refused issuance advanced Authority floor")
				}
				remaining, err := os.ReadDir(vault.records)
				if err != nil {
					t.Fatal(err)
				}
				if len(remaining) != len(records) {
					t.Fatal("refused issuance wrote successor record")
				}
				if kind == AuthorityAdmission {
					path, err := admissionLedgerPath(vault.root, made.RecordID)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := os.Stat(filepath.Clean(path)); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("refused issuance wrote ledger: %v", err)
					}
				}
			})
		}
	}
}
