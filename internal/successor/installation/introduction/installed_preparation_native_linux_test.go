//go:build linux && introduction_installed_native

package introduction_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/custody"
)

// This preparation verifies the actual input owners for the installed exchange.
// It starts no installed role and supplies no registration or delivery ACK.
func TestIntroductionInstalledNativePreparation(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: root-only canonical preparation required")
	}
	command := os.Getenv("ARDENTS_INTRODUCTION_CONTROL_PROGRAM")
	raw, err := os.ReadFile(command)
	pin := sha256.Sum256(raw)
	if err != nil || !filepath.IsAbs(command) || hex.EncodeToString(pin[:]) != os.Getenv("ARDENTS_INTRODUCTION_CONTROL_PROGRAM_SHA256") {
		t.Fatal("invalid environment: independently pinned canonical Control required", err)
	}
	parent := t.TempDir()
	vault, created, secrets := prepareIntroductionPermissionAuthority(t, parent)
	bundle, profile, authority, registrationPeer, submissionPeer := prepareIntroductionNativeCohort(t, command, parent, created.AdmissionAuthority.Public)
	prepared := prepareIntroductionStock(t, parent, parent, true, true, vault, created, secrets, bundle, profile, authority, registrationPeer, submissionPeer)
	if prepared.registrationPeer.NodeID == prepared.submissionPeer.NodeID {
		t.Fatal("independent adjacent purposes collapsed")
	}
	// Each Take durably consumes a different existing right. Exhaustion cannot
	// recreate stock; later receiving refusal cannot refund these presentations.
	seen := map[[32]byte]bool{}
	for _, batch := range []struct {
		class admission.Class
		count int
	}{{admission.RegistrationClass, 4}, {admission.ControlClass, 8}} {
		for index := 0; index <= batch.count; index++ {
			var nonce [32]byte
			if _, err := rand.Read(nonce[:]); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().UTC().Add(15 * time.Second).Truncate(time.Second)
			if deadline.After(prepared.deadline) {
				deadline = prepared.deadline
			}
			presentation := stock.Presentation{NetworkID: prepared.facts.NetworkID, StateGeneration: prepared.facts.StateGeneration, StateDigest: prepared.facts.StateDigest, ProfileDigest: prepared.facts.Digest, RecipientNodeID: prepared.receiver.NodeID, RecipientDutyGeneration: prepared.receiver.DutyGeneration, ChannelNonce: nonce, Deadline: deadline}
			token, err := prepared.holder.Take(t.Context(), presentation, uint8(batch.class))
			if index == batch.count {
				if err == nil || len(token) != 0 {
					clear(token)
					t.Fatal("exhausted original stock produced another right")
				}
				continue
			}
			if err != nil || len(token) == 0 {
				t.Fatal("genuine original presentation", err)
			}
			digest := sha256.Sum256(token)
			clear(token)
			if seen[digest] {
				t.Fatal("presentations reused one right")
			}
			seen[digest] = true
		}
	}
	first, repeated := prepared.issueCurrent(), prepared.issueCurrent()
	if _, outcome := first.Status(); outcome != "already-issued" {
		t.Fatal("issuance result lost", outcome)
	}
	if _, outcome := repeated.Status(); outcome != "already-issued" || !bytes.Equal(first.Response, repeated.Response) {
		t.Fatal("exact issuance retry changed retained response", outcome)
	}
	t.Log("genuine canonical signed cohort, separate Custody/Allocation authority, finite issuer result and twelve irreversible presentations; no installed runtime acceptance")
}

func prepareIntroductionPermissionAuthority(t *testing.T, parent string) (*custody.Vault, custody.Receipt, introductionTokenTestSecrets) {
	t.Helper()
	password := make([]byte, 32)
	if _, err := rand.Read(password); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(password) })
	secret := []byte(hex.EncodeToString(password))
	t.Cleanup(func() { clear(secret) })
	vault, err := custody.Open(custody.VaultConfig{Root: filepath.Join(parent, "permission-vault")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := vault.Close(); err != nil {
			t.Error(err)
		}
	})
	secrets := introductionTokenTestSecrets{secret}
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateAdmissionAuthority, Binding: custody.AuthorityBinding{Environment: sha256.Sum256([]byte("bounded installed Introduction exchange")), Network: [32]byte{1}, Root: sha256.Sum256([]byte("separate Introduction Permission authority")), Kind: custody.AuthorityAdmission}}, secrets)
	if err != nil {
		t.Fatal(err)
	}
	return vault, created, secrets
}
