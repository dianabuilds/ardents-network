//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/custody"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func prepareTextIssuancePermission(t *testing.T, owner *textContext, source *textSourceStateFixture) string {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	return prepareTextIssuancePermissionWithIdentity(t, owner, source, private, [3]uint32{0, 32, 0})
}

func prepareTextIssuancePermissionWithIdentity(t *testing.T, owner *textContext, source *textSourceStateFixture, private ed25519.PrivateKey, maxima [3]uint32) string {
	t.Helper()
	public := private.Public().(ed25519.PublicKey)
	vault, err := custody.Open(custody.VaultConfig{Root: t.TempDir(), Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := vault.Close(); err != nil {
			t.Error(err)
		}
	})
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateAdmissionAuthority,
		Authority: custody.AuthorityState{Binding: custody.AuthorityBinding{Environment: fixtureID(231), Network: owner.endpoint.network, Root: fixtureID(232), Kind: custody.AuthorityAdmission}}}, textPermissionSecretFixture{})
	if err != nil {
		t.Fatal(err)
	}

	source.mu.Lock()
	source.view.Profile.IssuanceAuthorityKey = created.AdmissionAuthority.Public
	profile := source.view.Profile
	source.mu.Unlock()
	issuerRoot := t.TempDir()
	if err := os.Chmod(issuerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	receipt, err := credential.InitializeClosedIssuerRoot(credential.ClosedIssuerRootConfig{Root: issuerRoot, NetworkID: profile.NetworkID, NodeID: profile.IssuerNodeID,
		IdentityKey: private, NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := credential.DecodeClosedIssuerProfile(receipt.Profile, public)
	if err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	source.view.Profile.TokenKeyCount = uint8(len(inventory.Keys))
	for index, key := range inventory.Keys {
		source.view.Profile.TokenKeys[index].WindowStart, source.view.Profile.TokenKeys[index].Class = key.WindowStart, uint8(key.Class)
		copy(source.view.Profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	source.mu.Unlock()
	source.issueRawPermission = func(t *testing.T, raw []byte, digest [32]byte) []byte {
		t.Helper()

		issued, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationIssueAdmissionPermission, RecordID: created.RecordID, Expected: created.Authority.Binding,
			AdmissionRequest: raw, AdmissionRequestCommitment: digest}, textPermissionSecretFixture{})
		if err != nil {
			t.Fatal(err)
		}
		return issued.AdmissionPermission
	}
	source.issuePermission = func(t *testing.T, owner *textContext, maxima [3]uint32) {
		t.Helper()
		raw, digest, err := owner.requestTextPermission(maxima)
		if err != nil {
			t.Fatal(err)
		}
		if err := owner.importTextPermission(digest, source.issueRawPermission(t, raw, digest)); err != nil {
			t.Fatal(err)
		}
	}
	source.issuePermission(t, owner, maxima)
	return issuerRoot
}

// A real offline Custody allocation and real blind batch reach the selected
// network opener. No server is listening in this failure fixture: it verifies
// ownership of failed/retried requests, not successful network qualification.
func TestTextIssuanceRetainsExactPendingBatchAcrossFailedAttempts(t *testing.T) {
	_, owner, source := textSourceContextFixture(t)
	prepareTextIssuancePermission(t, owner, source)
	receiver := source.view.Nodes[0].NodeID
	attempt := func() error {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		return owner.issueTextTokens(ctx, [][32]byte{receiver}, 2)
	}
	if err := attempt(); err == nil {
		t.Fatal("unavailable network issued tokens")
	}
	owner.mu.Lock()
	pending := owner.permission.pending
	if pending == nil {
		owner.mu.Unlock()
		t.Fatal("failed exchange lost prepared batch")
	}
	original := pending.pending.Request()
	selection := pending.selection
	owner.mu.Unlock()
	if err := attempt(); err == nil {
		t.Fatal("unavailable retry issued tokens")
	}
	owner.mu.Lock()
	if owner.permission.pending != pending || !bytes.Equal(original, pending.pending.Request()) || pending.selection != selection || owner.permission.batches != 1 || len(owner.permission.stock) != 0 || owner.issuance != nil {
		owner.mu.Unlock()
		t.Fatal("retry rotated blinding state, source, allowance, or left live flight")
	}
	owner.mu.Unlock()
	if err := owner.issueTextTokens(t.Context(), [][32]byte{source.view.Nodes[1].NodeID}, 2); err == nil {
		t.Fatal("retry changed receiving challenge")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if owner.permission != nil || len(pending.pending.Request()) != 0 {
		t.Fatal("context loss retained secret batch")
	}
}

func TestTextIssuanceRequiresPermissionBeforeSelectingAnyPeers(t *testing.T) {
	endpoint, owner, source := textSourceContextFixture(t)
	if err := owner.issueTextTokens(t.Context(), [][32]byte{source.view.Nodes[0].NodeID}, 2); err == nil {
		t.Fatal("missing permission admitted")
	}
	if endpoint.closedEntries != nil || owner.sourceSet != nil {
		t.Fatal("unallocated owner selected peers")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := owner.issueTextTokens(canceled, [][32]byte{source.view.Nodes[0].NodeID}, 2); err == nil {
		t.Fatal("canceled caller admitted")
	}
	if endpoint.closedEntries != nil {
		t.Fatal("canceled caller claimed Entry root")
	}
}

func TestTextPrefixPreparesBothReceiversInOneRetryableBatch(t *testing.T) {
	_, owner, source := textSourceContextFixture(t)
	prepareTextIssuancePermission(t, owner, source)
	attempt := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		prefix, err := owner.openTextPrefix(ctx)
		if prefix != nil || err == nil {
			if prefix != nil {
				_ = prefix.Close()
			}
			t.Fatal("unavailable issuer created a prefix")
		}
	}
	attempt()
	owner.mu.Lock()
	pending := owner.permission.pending
	if pending == nil || len(pending.challenges) != 2 || owner.permission.batches != 1 {
		owner.mu.Unlock()
		t.Fatal("cold prefix did not retain one two-receiver batch")
	}
	if pending.challenges[0].ReceiverNodeID != pending.selection.EntryNodeID ||
		pending.challenges[1].ReceiverNodeID != pending.selection.InteriorNodeID ||
		pending.challenges[0].Class != 2 || pending.challenges[1].Class != 2 {
		owner.mu.Unlock()
		t.Fatal("prefix batch does not match private source selection")
	}
	original := pending.pending.Request()
	owner.mu.Unlock()
	request, err := credential.DecodeClosedTokenBatch(original)
	if err != nil || len(request.BlindedRequests) != 2 || request.Class != 2 {
		t.Fatalf("cold prefix request: %v", err)
	}
	attempt()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.permission.pending != pending || owner.permission.batches != 1 || !bytes.Equal(original, pending.pending.Request()) ||
		owner.issuance != nil || owner.prefixOpening != nil || len(owner.permission.stock) != 0 {
		t.Fatal("retry changed receiver ordering, blinding, batch debit or lifecycle")
	}
}

func TestTextIssuerStockRetryRetainsOriginalInternalBatch(t *testing.T) {
	_, owner, source := textSourceContextFixture(t)
	certificate, _ := testCertificate(t, 391, "issuer-stock-retry")
	private := certificate.PrivateKey.(ed25519.PrivateKey)
	defer clear(private)
	prepareTextIssuancePermissionWithIdentity(t, owner, source, private, [3]uint32{4, 8, 0})
	attempt := func() error {
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		return owner.prepareTextIssuerStock(ctx, nil, 0, nil)
	}
	if err := attempt(); err == nil {
		t.Fatal("unavailable issuer unexpectedly funded stock")
	}
	owner.mu.Lock()
	batch := owner.permission.pending
	if batch == nil {
		owner.mu.Unlock()
		t.Fatal("internal batch lost on failure")
	}
	original := batch.pending.Request()
	owner.mu.Unlock()
	defer clear(original)
	if err := attempt(); err == nil {
		t.Fatal("pending internal batch was skipped on stock retry")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.permission.pending != batch || !bytes.Equal(original, batch.pending.Request()) ||
		owner.permission.batches != 1 || owner.permission.reserved != [3]uint32{4, 0, 0} {
		t.Fatal("internal stock retry changed request or repeated allocation")
	}
}
