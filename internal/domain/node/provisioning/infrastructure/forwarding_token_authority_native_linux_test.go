//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/custody"
	networkdomain "github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// Only the test front-end supplies this fresh in-memory password. Production
// terminal custody is unchanged; neither a private authority nor a generic
// signing capability is supplied to this harness.
type forwardingTokenTestSecrets struct{ password []byte }

func (s forwardingTokenTestSecrets) ReadSecret(ctx context.Context, prompt custody.SecretPrompt) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch prompt {
	case custody.SecretPromptVaultCreate, custody.SecretPromptVaultCreateConfirm, custody.SecretPromptVaultUnlock:
		return bytes.Clone(s.password), nil
	}
	return nil, errors.New("unexpected test Custody purpose")
}
func (forwardingTokenTestSecrets) Confirm(context.Context, custody.ConfirmationPrompt) (bool, error) {
	return false, errors.New("no test replacement or purge approval")
}

func TestForwardingTokenAuthorityNativeGenuineCustodyStateAndStock(t *testing.T) {
	parent := networkfixture.ProtectedExt4Parent(t)
	command, _ := networkfixture.PinnedProvisionProgram(t, "ARDENTS_INFRASTRUCTURE_PROVISION_PROGRAM")
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	password := []byte(hex.EncodeToString(secret))
	clear(secret)
	defer clear(password)
	secrets := forwardingTokenTestSecrets{password}
	vault, err := custody.Open(custody.VaultConfig{Root: filepath.Join(parent, "permission-vault")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := vault.Close(); err != nil {
			t.Error(err)
		}
	})
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateAdmissionAuthority, Binding: custody.AuthorityBinding{Environment: sha256.Sum256([]byte("bounded native token preparation")), Network: [32]byte{1}, Root: sha256.Sum256([]byte("separate Admission permission test authority")), Kind: custody.AuthorityAdmission}}, secrets)
	if err != nil || created.AdmissionAuthority.Public == [32]byte{} {
		t.Fatal("genuine encrypted Admission authority creation", err)
	}
	bundleRaw, profileRaw, statePublic := prepareForwardingStateNativeInputsWithPermission(t, command, parent, created.AdmissionAuthority.Public)
	prepared := prepareForwardingIssuedStock(t, parent, vault, created, secrets, bundleRaw, profileRaw, statePublic, 3)
	holder, receiver, facts, deadline := prepared.holder, prepared.receiver, prepared.facts, prepared.deadline
	observe := prepared.observe
	for index := 0; index < 2; index++ {
		var nonce [32]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			t.Fatal(err)
		}
		presented, err := holder.Take(t.Context(), stock.Presentation{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest, ProfileDigest: facts.Digest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, ChannelNonce: nonce, Deadline: deadline}, uint8(admission.ForwardClass))
		if err != nil {
			t.Fatal(err)
		}
		current, at, err := observe()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := receiving.VerifyToken(current, receiver, admission.ForwardClass, presented, at); err != nil {
			t.Fatal("actual blind-issued token verification", index, err)
		}
		wrong := receiver
		wrong.NodeID = [32]byte{9}
		if _, err := receiving.VerifyToken(current, wrong, admission.ForwardClass, presented, at); err == nil {
			t.Fatal("foreign receiver accepted genuine token")
		}
		changed := bytes.Clone(presented)
		changed[len(changed)-1] ^= 1
		if _, err := receiving.VerifyToken(current, receiver, admission.ForwardClass, changed, at); err == nil {
			t.Fatal("damaged token accepted")
		}
	}
	// One genuine token remains, so revocation cannot be confused with empty
	// stock. Retired State must deny its presentation and any retained response.
	if err := prepared.retireState(); err != nil {
		t.Fatal(err)
	}
	if tokenBytes, err := holder.Take(t.Context(), stock.Presentation{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest, ProfileDigest: facts.Digest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, ChannelNonce: [32]byte{8}, Deadline: deadline}, uint8(admission.ForwardClass)); err == nil || len(tokenBytes) != 0 {
		t.Fatal("retired State supplied remaining token")
	}
	denied := prepared.issueCurrent()
	if _, outcome := denied.Status(); outcome != "authority-unavailable" || len(denied.Response) != 0 {
		t.Fatal("retired State supplied retained issuer response", denied.Phase, outcome)
	}
}

// This private test composition retains genuine owners. Returned callbacks are
// original State/Issuer operations, never supplied accepting authority facts.
type forwardingPreparedStock struct {
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
}

func prepareForwardingIssuedStock(t *testing.T, parent string, vault *custody.Vault, created custody.Receipt, secrets forwardingTokenTestSecrets, bundleRaw, profileRaw []byte, statePublic [32]byte, count uint32, peers ...[32]byte) forwardingPreparedStock {
	t.Helper()
	if count < 1 || count > 8 && !(count == 16 && len(peers) == 1) {
		t.Fatal("finite test Forward presentation count required")
	}
	if len(peers) > 1 || len(peers) == 1 && count != 8 && count != 16 {
		t.Fatal("one explicit peer requires a fixed eight- or sixteen-credential cohort")
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
	if _, err := owner.AcceptEpoch(t.Context(), bundle.Epoch, bundle.Inputs, bundle.Materials); err != nil {
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
	var inventory nodeTokenInventory
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
	request, commitment, err := holder.Request([3]uint32{0, count, 0})
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
	if err != nil || member.RoleDomain != 4 || member.Subrole != 1 {
		t.Fatal("actual receiver duty", err)
	}
	var peer networkdomain.Member
	if len(peers) == 1 {
		peer, err = view.Member(peers[0], view.ObservedAt())
		if err != nil || peer.RoleDomain != member.RoleDomain || peer.Subrole != 2 || peer.NodeID == member.NodeID || peer.PublicKey == member.PublicKey || peer.FamilyID == member.FamilyID {
			t.Fatal("actual distinct Interior receiver duty", err)
		}
	}
	deadline := time.Now().UTC().Add(30 * time.Second).Truncate(time.Second)
	permissionHourEnd := now.UTC().Truncate(time.Hour).Add(time.Hour)
	if deadline.After(permissionHourEnd) {
		deadline = permissionHourEnd
	}
	challenge := token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration, Class: uint8(admission.ForwardClass), WindowStart: now.Truncate(time.Hour)}
	var exchange [32]byte
	if _, err := rand.Read(exchange[:]); err != nil {
		t.Fatal(err)
	}
	challenges := make([]token.ClosedTokenContext, count)
	for index := range challenges {
		challenges[index] = challenge
		if peer.NodeID != [32]byte{} && index >= int(count/2) {
			challenges[index].ReceiverNodeID = peer.NodeID
			challenges[index].ReceiverDutyGeneration = peer.DutyGeneration
		}
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
	return forwardingPreparedStock{holder: holder, receiver: receiver, facts: facts, deadline: deadline, observe: observe, retireState: owner.Close, endpoint: member.Endpoint, publicKey: member.PublicKey, peer: peer, issueCurrent: func() issuer.Result {
		return issuer.IssueCurrent(t.Context(), plan, batch, quota.Bootstrap, issuerObserve)
	}}
}
