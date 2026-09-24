//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/tokenjournal"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// A real Custody permission and actual issuer produce the stock fixture.
// The qualified-worker/State and in-flight-opening seams are explicit fixtures;
// this test verifies the Endpoint's real stock-to-journal consumer.
func TestTextTokenPresentationBurnsStockBeforeReturningBytes(t *testing.T) {
	endpoint, owner, selection, _, hello, original := textTokenPresentationFixture(t)
	defer clear(original)
	flightContext, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	close(done)
	opening := &textPrefixOpeningOperation{owner: owner, context: flightContext, cancelOperation: cancel, done: done}
	owner.source.opening = opening
	returned, err := opening.presentTextToken(selection, hello, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(returned)
	if !bytes.Equal(returned, original) || len(owner.permission.stock[0].tokens) != 0 {
		t.Fatal("stock transfer wrong")
	}
	raw, err := os.ReadFile(filepath.Join(endpoint.closedTokenRoot, "attempts"))
	if err != nil || len(raw) != textTokenReceiptHeaderSize+textTokenReceiptSize || bytes.Contains(raw, original) {
		t.Fatal("missing durable receipt or leaked token")
	}
	receipts := parseTextTokenReceipts(t, raw, endpoint.network)
	if len(receipts) != 1 || receipts[0].attempt != hello.ChannelNonce || receipts[0].receiver != hello.RecipientNodeID {
		t.Fatal("receipt lost attempt binding")
	}
	hello.ChannelNonce[0]++
	if token, err := opening.presentTextToken(selection, hello, 2); err == nil || len(token) != 0 {
		t.Fatal("stock replayed")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := tokenjournal.Open(endpoint.closedTokenRoot, endpoint.network, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Mark(original, journalAttemptFromReceipt(receipts[0])); err == nil {
		t.Fatal("context restart revived spent token")
	}
}

// Invalid issuer stock is consumed locally and never becomes a durable attempt
// or bytes presented to Route.
func TestTextTokenInvalidStockCannotReachJournal(t *testing.T) {
	endpoint, owner, _, profile, hello, original := textTokenPresentationFixture(t)
	defer clear(original)
	owner.mu.Lock()
	owner.permission.stock[0].tokens[0][0] ^= 0xff
	returned, err := owner.takeTextTokenLocked(profile, time.Now().UTC(), hello, 2, t.Context())
	remaining := len(owner.permission.stock[0].tokens)
	owner.mu.Unlock()
	if err == nil || len(returned) != 0 || remaining != 0 || textTokenTransferFailureStage(err) != "verification" {
		t.Fatalf("invalid stock transfer: bytes=%d remaining=%d stage=%s err=%v", len(returned), remaining, textTokenTransferFailureStage(err), err)
	}
	if _, err := os.Stat(filepath.Join(endpoint.closedTokenRoot, "attempts")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid stock reached durable journal: %v", err)
	}
}

func TestTextTokenCancellationAfterDurableMarkRetainsBurn(t *testing.T) {
	endpoint, owner, _, profile, hello, original := textTokenPresentationFixture(t)
	defer clear(original)
	attempt, cancel := context.WithCancel(t.Context())
	cancel()
	owner.mu.Lock()
	returned, err := owner.takeTextTokenLocked(profile, time.Now().UTC(), hello, 2, attempt)
	remaining := len(owner.permission.stock[0].tokens)
	owner.mu.Unlock()
	if err == nil || len(returned) != 0 || remaining != 0 || textTokenTransferFailureStage(err) != "owner" {
		t.Fatalf("cancelled durable spend returned token or wrong result: bytes=%d remaining=%d stage=%s err=%v",
			len(returned), remaining, textTokenTransferFailureStage(err), err)
	}
	receipts := readTextTokenReceipts(t, endpoint.closedTokenRoot, endpoint.network)
	if len(receipts) != 1 || receipts[0].attempt != hello.ChannelNonce {
		t.Fatal("cancelled spend lost its durable attempt receipt")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := tokenjournal.Open(endpoint.closedTokenRoot, endpoint.network, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Mark(original, journalAttemptFromReceipt(receipts[0])); err == nil {
		t.Fatal("restart revived token burned before cancellation")
	}
}

func textTokenPresentationFixture(t *testing.T) (*endpoint, *textContext, route.ClosedBootstrapSelection,
	state.ClosedProfileView, route.ClosedHello, []byte) {
	t.Helper()
	endpoint, owner, source := textSourceContextFixture(t)
	root := prepareTextIssuancePermission(t, owner, source)
	endpoint.closedTokenRoot = t.TempDir()
	selection := selectTextSource(t, owner)
	profile := source.view.Profile
	duty := uint64(0)
	for _, node := range source.view.Nodes[:source.view.NodeCount] {
		if node.NodeID == selection.EntryNodeID {
			duty = node.DutyGeneration
		}
	}
	challenge := credential.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
		ReceiverNodeID: selection.EntryNodeID, ReceiverDutyGeneration: duty, Class: 2, WindowStart: owner.permission.accepted.NotBefore}
	pending, err := credential.PrepareClosedTokenBatch(credential.ClosedTokenBatchConfig{Profile: profile, Contexts: []credential.ClosedTokenContext{challenge},
		Permission: owner.permission.accepted, HolderKey: owner.permission.holder, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := credential.OpenClosedTokenIssuer(credential.ClosedTokenIssuerConfig{Root: root, NetworkID: profile.NetworkID,
		CurrentProfile: func() (state.ClosedProfileView, bool) { return profile, true }, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	nonce := [32]byte{241}
	operation, err := route.EncodeClosedIssuanceRequest(nonce, pending.Request())
	if err != nil {
		t.Fatal(err)
	}
	response, err := issuer.IssueTerminalOperation(operation)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := pending.FinalizeTerminalOperation(nonce, response)
	if err != nil || len(tokens) != 1 {
		t.Fatalf("fixture issuance: %v", err)
	}
	if err := issuer.Close(); err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(tokens[0])
	owner.permission.stock = []textTokenStock{{challenge: challenge, tokens: tokens}}
	hello := route.ClosedHello{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest,
		ProfileDigest: profile.Digest, RecipientNodeID: challenge.ReceiverNodeID, RecipientDutyGeneration: duty,
		Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{242}, Deadline: profile.NotAfter}
	return endpoint, owner, selection, profile, hello, original
}
