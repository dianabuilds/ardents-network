//go:build linux && descriptor_installed_native

package descriptor_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/custody"
	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// Only the test front-end supplies this fresh in-memory password. Production
// terminal custody is unchanged; neither a private authority nor a generic
// signing capability is supplied to this harness.
type descriptorTokenTestSecrets struct{ password []byte }

func (s descriptorTokenTestSecrets) ReadSecret(ctx context.Context, prompt custody.SecretPrompt) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch prompt {
	case custody.SecretPromptVaultCreate, custody.SecretPromptVaultCreateConfirm, custody.SecretPromptVaultUnlock:
		return bytes.Clone(s.password), nil
	}
	return nil, errors.New("unexpected test Custody purpose")
}
func (descriptorTokenTestSecrets) Confirm(context.Context, custody.ConfirmationPrompt) (bool, error) {
	return false, errors.New("no test replacement or purge approval")
}

// This private test composition retains genuine owners. Returned callbacks are
// original State/Issuer operations, never supplied accepting authority facts.
type descriptorPreparedStock struct {
	holder       *stock.Owner
	receiver     receiving.Receiver
	facts        admission.AuthorityFacts
	deadline     time.Time
	observe      func() (admission.AuthorityFacts, time.Time, error)
	retireState  func() error
	issueCurrent func() issuer.Result
	endpoint     string
	publicKey    [32]byte
	peer         networkdomain.Member
	current      func() (networkdomain.RuntimeView, error)
}

func prepareDescriptorIssuedStock(t *testing.T, parent string, vault *custody.Vault, created custody.Receipt, secrets descriptorTokenTestSecrets, bundleRaw, profileRaw []byte, statePublic [32]byte, count uint32, peerID [32]byte) descriptorPreparedStock {
	t.Helper()
	// Every operation consumes one genuine Control right, including refusals.
	// Actual Allocation, issuer quota and receiving limits remain authoritative.
	if count != 16 {
		t.Fatal("one finite sixteen-operation Descriptor campaign required")
	}
	if statePublic == created.AdmissionAuthority.Public {
		t.Fatal("State and Permission keys collapsed")
	}
	owner, err := state.Open(state.Config{Root: filepath.Join(parent, "client-state"), NetworkID: [32]byte{1}, Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(statePublic[:]): statePublic[:]}, Threshold: 1, ClosedProfileAuthority: statePublic[:], AcceptedProfile: "ardents-route-v3", Clock: time.Now, ObserveClock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	bundle, err := source.DecodeBundle(bundleRaw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Accept(t.Context(), bundle.Epoch, bundle.Inputs, bundle.Materials); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.AcceptClosedProfile(profileRaw); err != nil {
		t.Fatal(err)
	}
	// Each call gets one real fresh State observation. There is no supplied
	// accepting profile callback or cached current-authority flag.
	project := func(view networkdomain.RuntimeView) (admission.AuthorityFacts, time.Time, error) {
		profile := view.Profile()
		facts := admission.AuthorityFacts{NetworkID: profile.Network, StateGeneration: profile.Generation, StateDigest: profile.EpochDigest, Digest: profile.Digest, IssuanceAuthorityKey: profile.IssuanceAuthorityKey, IssuerNodeID: profile.IssuerNodeID, IssuerDutyGeneration: profile.IssuerDutyGeneration, Epoch: profile.Epoch, NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, TokenKeyCount: uint8(len(profile.TokenKeys))}
		if len(profile.TokenKeys) > len(facts.TokenKeys) {
			return facts, time.Time{}, errors.New("token key inventory exceeds bound")
		}
		for index, key := range profile.TokenKeys {
			facts.TokenKeys[index] = admission.TokenKey{WindowStart: key.WindowStart, Class: key.Class, SPKI: key.SPKI}
		}
		now := view.ObservedAt()
		return facts, now, facts.ValidateAt(now)
	}
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		view, err := owner.CurrentRuntime()
		if err != nil {
			return admission.AuthorityFacts{}, time.Time{}, err
		}
		return project(view)
	}
	facts, now, err := observe()
	if err != nil || facts.IssuanceAuthorityKey != created.AdmissionAuthority.Public {
		t.Fatal("actual State-bound Permission authority", err)
	}
	var inventory descriptorTokenInventory
	raw, err := os.ReadFile(filepath.Join(parent, "canonical-preparation", "token-material", "public", "token-inventory.json"))
	if err != nil || json.Unmarshal(raw, &inventory) != nil {
		t.Fatal("actual canonical public key inventory", err)
	}
	plan := issuer.Plan{AdmissionRoot: filepath.Join(parent, "issuance-quota"), KeyRoot: filepath.Join(parent, "canonical-preparation", "token-material", "material"), KeyBinding: inventory.Inventory.Binding, ResultRoot: filepath.Join(parent, "issuance-results"), AdmissionBinding: quota.LedgerBinding{Network: facts.NetworkID, Issuer: facts.IssuerNodeID, Authority: facts.IssuanceAuthorityKey, Profile: facts.Digest, Duty: facts.IssuerDutyGeneration, Start: facts.NotBefore, End: facts.NotAfter, Keys: inventory.Inventory.Keys}}
	if err := quota.Initialize(plan.AdmissionRoot, plan.AdmissionBinding); err != nil {
		t.Fatal(err)
	}
	initialized := issuer.Initialize(t.Context(), plan)
	if _, outcome := initialized.Status(); outcome != "initialized-results" {
		t.Fatal("actual finite result initialization", initialized.Phase, outcome)
	}
	holderRoot := filepath.Join(parent, "holder")
	if err := os.Mkdir(holderRoot, 0700); err != nil {
		t.Fatal(err)
	}
	holder, err := stock.Open(holderRoot, admission.AllocationUser, observe)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := holder.Close(); err != nil {
			t.Error(err)
		}
	})
	request, commitment, err := holder.Request([3]uint32{count, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	operation := custody.Operation{Kind: custody.OperationIssueAdmissionPermission, RecordID: created.RecordID, Expected: created.Authority.Binding, AdmissionRequest: request, AdmissionRequestCommitment: sha256.Sum256(request)}
	permission, err := vault.Execute(t.Context(), operation, secrets)
	if err != nil || len(permission.AdmissionPermission) != 228 {
		t.Fatal("actual Custody/Allocation permission", err)
	}
	repeatedPermission, err := vault.Execute(t.Context(), operation, secrets)
	if err != nil || !bytes.Equal(permission.AdmissionPermission, repeatedPermission.AdmissionPermission) || permission.Authority.Generation != repeatedPermission.Authority.Generation || permission.Authority.Generation <= created.Authority.Generation {
		t.Fatal("encrypted exact permission repeat/floor", err)
	}
	if err := holder.Import(commitment, permission.AdmissionPermission); err != nil {
		t.Fatal(err)
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member([32]byte{3}, view.ObservedAt())
	if err != nil || member.RoleDomain != 2 || member.Subrole != 5 {
		t.Fatal("actual receiver duty", err)
	}
	peer, err := view.Member(peerID, view.ObservedAt())
	if err != nil || peer.RoleDomain != 1 || peer.Subrole != 2 || peer.FamilyID == member.FamilyID || peer.PublicKey == member.PublicKey {
		t.Fatal("actual distinct Source Interior", err)
	}
	deadline := time.Now().UTC().Add(30 * time.Second).Truncate(time.Second)
	permissionHourEnd := now.UTC().Truncate(time.Hour).Add(time.Hour)
	if deadline.After(permissionHourEnd) {
		deadline = permissionHourEnd
	}
	challenge := token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration, Class: uint8(admission.ControlClass), WindowStart: now.Truncate(time.Hour)}
	var exchange [32]byte
	if _, err := rand.Read(exchange[:]); err != nil {
		t.Fatal(err)
	}
	challenges := make([]token.ClosedTokenContext, count)
	for index := range challenges {
		challenges[index] = challenge

	}
	attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: exchange, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: deadline})
	if err != nil {
		t.Fatal(err)
	}
	batch, _, err := attempt.Request()
	if err != nil {
		t.Fatal(err)
	}
	issuerObserve := func() (admission.AuthorityFacts, time.Time, error) {
		view, err := owner.CurrentRuntime()
		if err != nil {
			return admission.AuthorityFacts{}, time.Time{}, err
		}
		current, at, err := project(view)
		if err != nil {
			return current, at, err
		}
		issuerMember, err := view.Member(current.IssuerNodeID, view.ObservedAt())
		if err != nil || issuerMember.PublicKey != plan.KeyBinding.Signer || issuerMember.DutyGeneration != current.IssuerDutyGeneration || issuerMember.RoleDomain != 2 || issuerMember.Subrole != 6 {
			return current, at, errors.Join(errors.New("original issuer duty unavailable"), err)
		}
		return current, at, nil
	}
	issued := issuer.IssueCurrent(t.Context(), plan, batch, quota.Bootstrap, issuerObserve)
	if _, outcome := issued.Status(); outcome != "issued-offline" {
		t.Fatal("genuine finite issuance", issued.Phase, outcome)
	}
	repeated := issuer.IssueCurrent(t.Context(), plan, batch, quota.Bootstrap, issuerObserve)
	if _, outcome := repeated.Status(); outcome != "already-issued" || !bytes.Equal(issued.Response, repeated.Response) {
		t.Fatal("actual issuance exact retry", repeated.Phase, outcome)
	}
	if err := attempt.Complete(issued.Response, nil); err != nil {
		t.Fatal(err)
	}
	receiver := receiving.Receiver{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest, ProfileDigest: facts.Digest, NodeID: member.NodeID, DutyGeneration: member.DutyGeneration}
	return descriptorPreparedStock{holder: holder, receiver: receiver, facts: facts, deadline: deadline, observe: observe, retireState: owner.Close, endpoint: member.Endpoint, publicKey: member.PublicKey, peer: peer, current: owner.CurrentRuntime, issueCurrent: func() issuer.Result {
		return issuer.IssueCurrent(t.Context(), plan, batch, quota.Bootstrap, issuerObserve)
	}}
}

type descriptorTokenInventory struct {
	Inventory issuance.Inventory `json:"inventory"`
}
