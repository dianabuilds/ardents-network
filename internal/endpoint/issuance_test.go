//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/custody"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func prepareIssuancePermission(t *testing.T, owner *dutyContext, source *sourceStateFixture) string {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	return prepareIssuancePermissionWithIdentity(t, owner, source, private, [3]uint32{0, 32, 0})
}

func prepareIssuancePermissionWithIdentity(t *testing.T, owner *dutyContext, source *sourceStateFixture, private ed25519.PrivateKey, maxima [3]uint32) string {
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
		Authority: custody.AuthorityState{Binding: custody.AuthorityBinding{Environment: fixtureID(231), Network: owner.endpoint.network, Root: fixtureID(232), Kind: custody.AuthorityAdmission}}}, permissionSecretFixture{})
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
	inventory, err := admission.DecodeClosedIssuerProfile(receipt.Profile, public)
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
			AdmissionRequest: raw, AdmissionRequestCommitment: digest}, permissionSecretFixture{})
		if err != nil {
			t.Fatal(err)
		}
		return issued.AdmissionPermission
	}
	source.issuePermission = func(t *testing.T, owner *dutyContext, maxima [3]uint32) {
		t.Helper()
		raw, digest, err := owner.requestPermission(maxima)
		if err != nil {
			t.Fatal(err)
		}
		if err := owner.importPermission(digest, source.issueRawPermission(t, raw, digest)); err != nil {
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
	_, owner, source := sourceContextFixture(t)
	prepareIssuancePermission(t, owner, source)
	receiver := source.view.Nodes[0].NodeID
	attempt := func() error {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		return owner.issueTokens(ctx, [][32]byte{receiver}, 2)
	}
	if err := attempt(); err == nil {
		t.Fatal("unavailable network issued tokens")
	}
	owner.mu.Lock()
	pending := owner.tokens.permission.pending
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
	if owner.tokens.permission.pending != pending || !bytes.Equal(original, pending.pending.Request()) || pending.selection != selection || owner.tokens.permission.batches != 1 || len(owner.tokens.permission.stock) != 0 || owner.tokens.issuance != nil {
		owner.mu.Unlock()
		t.Fatal("retry rotated blinding state, source, allowance, or left live flight")
	}
	owner.mu.Unlock()
	if err := owner.issueTokens(t.Context(), [][32]byte{source.view.Nodes[1].NodeID}, 2); err == nil {
		t.Fatal("retry changed receiving challenge")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if owner.tokens.permission != nil || len(pending.pending.Request()) != 0 {
		t.Fatal("context loss retained secret batch")
	}
}

func TestTextPermissionRevocationDefersActiveBatchDiscardUntilOperationCompletion(t *testing.T) {
	_, owner, source := sourceContextFixture(t)
	prepareIssuancePermission(t, owner, source)
	receiver := source.view.Nodes[0].NodeID
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := owner.issueTokens(ctx, [][32]byte{receiver}, 2); err == nil {
		t.Fatal("unavailable network issued tokens")
	}

	owner.mu.Lock()
	permission := owner.tokens.permission
	batch := permission.pending
	operation := newIssuanceOperation(owner, permission, permission.profile, batch, false)
	owner.tokens.issuance = operation
	t.Cleanup(func() {
		select {
		case <-operation.done:
			return
		default:
		}
		operation.cancel()
		canceled, stop := context.WithCancel(context.Background())
		stop()
		_ = operation.complete(canceled, client.ClosedIssuanceExchangeResult{}, context.Canceled)
	})
	owner.clearPermissionLocked()
	if owner.tokens.permission != nil || !operation.discardPermission || len(batch.pending.Request()) == 0 {
		owner.mu.Unlock()
		t.Fatal("revocation did not detach permission while retaining the active operation batch")
	}
	owner.mu.Unlock()

	canceled, stop := context.WithCancel(t.Context())
	stop()
	if err := operation.complete(canceled, client.ClosedIssuanceExchangeResult{}, context.Canceled); err == nil {
		t.Fatal("revoked operation completed")
	}
	if len(batch.pending.Request()) != 0 {
		t.Fatal("completed revoked operation retained its secret batch")
	}
}

func TestTextRecoveryIssuanceCancellationDiscardsBatchWithoutRefund(t *testing.T) {
	_, owner, source := sourceContextFixture(t)
	prepareIssuancePermission(t, owner, source)
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	for index := 0; index < 2; index++ {
		source.snapshot.Candidates[index].Endpoint = listener.Addr().String()
	}
	receiver := source.view.Nodes[0].NodeID
	source.mu.Unlock()
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	workerDone := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
		_ = owner.Close()
		select {
		case <-workerDone:
		case <-time.After(time.Second):
			t.Error("recovery cancellation worker did not join during cleanup")
		}
	})
	go func() {
		defer close(workerDone)
		result <- owner.issueRecoveryTokens(ctx, [][32]byte{receiver}, 2)
	}()
	accepted, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer accepted.Close()
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("canceled recovery issuance succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("recovery cancellation did not join its request transport")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.tokens.issuance != nil || owner.tokens.permission.pending != nil || owner.tokens.permission.batches != 1 ||
		owner.tokens.permission.reserved != [3]uint32{0, 1, 0} || len(owner.tokens.permission.stock) != 0 {
		t.Fatalf("recovery cancellation state: issuance=%v pending=%v batches=%d reserved=%v stock=%d",
			owner.tokens.issuance != nil, owner.tokens.permission.pending != nil, owner.tokens.permission.batches, owner.tokens.permission.reserved, len(owner.tokens.permission.stock))
	}
}

func TestTextIssuanceRequiresPermissionBeforeSelectingAnyPeers(t *testing.T) {
	endpoint, owner, source := sourceContextFixture(t)
	if err := owner.issueTokens(t.Context(), [][32]byte{source.view.Nodes[0].NodeID}, 2); err == nil {
		t.Fatal("missing permission admitted")
	}
	if endpoint.closedEntries != nil || owner.source.set != nil {
		t.Fatal("unallocated owner selected peers")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := owner.issueTokens(canceled, [][32]byte{source.view.Nodes[0].NodeID}, 2); err == nil {
		t.Fatal("canceled caller admitted")
	}
	if endpoint.closedEntries != nil {
		t.Fatal("canceled caller claimed Entry root")
	}
}

func TestTextIssuanceRevocationBeforeDelayedCompletionJoinsTransport(t *testing.T) {
	_, owner, source := sourceContextFixture(t)
	prepareIssuancePermission(t, owner, source)
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	for index := 0; index < 2; index++ {
		source.snapshot.Candidates[index].Endpoint = listener.Addr().String()
	}
	receiver := source.view.Nodes[0].NodeID
	source.mu.Unlock()
	var workers sync.WaitGroup
	t.Cleanup(func() {
		_ = listener.Close()
		_ = owner.Close()
		joined := make(chan struct{})
		go func() {
			workers.Wait()
			close(joined)
		}()
		select {
		case <-joined:
		case <-time.After(time.Second):
			t.Error("revoked issuance workers did not join during cleanup")
		}
	})
	issued := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		issued <- owner.issueTokens(t.Context(), [][32]byte{receiver}, 2)
	}()
	accepted, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer accepted.Close()
	owner.mu.Lock()
	started := owner.tokens.issuance != nil && owner.tokens.permission != nil && owner.tokens.permission.pending != nil
	owner.mu.Unlock()
	if !started {
		t.Fatal("issuance transport opened before its operation was retained")
	}
	closed := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		closed <- owner.Close()
	}()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("context revoke did not join delayed issuance transport")
	}
	select {
	case err := <-issued:
		if err == nil {
			t.Fatal("revoked issuance returned usable tokens")
		}
	default:
		t.Fatal("context Close returned before issuance operation joined")
	}
	if err := accepted.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, accepted); err != nil {
		t.Fatalf("revoked issuance request transport did not close: %v", err)
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.tokens.issuance != nil || owner.tokens.permission != nil {
		t.Fatal("revoked issuance retained operation or usable token material")
	}
}

func TestTextPrefixPreparesBothReceiversInOneRetryableBatch(t *testing.T) {
	_, owner, source := sourceContextFixture(t)
	prepareIssuancePermission(t, owner, source)
	attempt := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		prefix, err := owner.openPrefix(ctx)
		if prefix != nil || err == nil {
			if prefix != nil {
				_ = prefix.Close()
			}
			t.Fatal("unavailable issuer created a prefix")
		}
	}
	attempt()
	owner.mu.Lock()
	pending := owner.tokens.permission.pending
	if pending == nil || len(pending.challenges) != 2 || owner.tokens.permission.batches != 1 {
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
	if owner.tokens.permission.pending != pending || owner.tokens.permission.batches != 1 || !bytes.Equal(original, pending.pending.Request()) ||
		owner.tokens.issuance != nil || owner.source.opening != nil || len(owner.tokens.permission.stock) != 0 {
		t.Fatal("retry changed receiver ordering, blinding, batch debit or lifecycle")
	}
}

func TestTextIssuerStockRetryRetainsOriginalInternalBatch(t *testing.T) {
	_, owner, source := sourceContextFixture(t)
	certificate, _ := testCertificate(t, 391, "issuer-stock-retry")
	private := certificate.PrivateKey.(ed25519.PrivateKey)
	defer clear(private)
	prepareIssuancePermissionWithIdentity(t, owner, source, private, [3]uint32{4, 8, 0})
	attempt := func() error {
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		return owner.prepareIssuerStock(ctx, nil, 0, nil, nil, nil)
	}
	if err := attempt(); err == nil {
		t.Fatal("unavailable issuer unexpectedly funded stock")
	}
	owner.mu.Lock()
	batch := owner.tokens.permission.pending
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
	if owner.tokens.permission.pending != batch || !bytes.Equal(original, batch.pending.Request()) ||
		owner.tokens.permission.batches != 1 || owner.tokens.permission.reserved != [3]uint32{4, 0, 0} {
		t.Fatal("internal stock retry changed request or repeated allocation")
	}
}
