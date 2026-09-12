//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// A real Custody permission and actual issuer produce the stock fixture.
// The qualified-worker/State and in-flight-opening seams are explicit fixtures;
// this test verifies the Endpoint's real stock-to-journal consumer.
func TestTextTokenPresentationBurnsStockBeforeReturningBytes(t *testing.T) {
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
		ReceiverNodeID: selection.EntryNodeID, ReceiverDutyGeneration: duty, Class: 2, WindowStart: profile.NotBefore}
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
	defer clear(original)
	owner.permission.stock = []textTokenStock{{challenge: challenge, tokens: tokens}}
	flightContext, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	close(done)
	owner.prefixOpening = &textSourceFlight{context: flightContext, cancel: cancel, done: done}
	hello := route.ClosedHello{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest,
		ProfileDigest: profile.Digest, RecipientNodeID: challenge.ReceiverNodeID, RecipientDutyGeneration: duty,
		Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{242}, Deadline: profile.NotAfter}
	returned, err := owner.presentTextToken(selection, hello, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(returned)
	if !bytes.Equal(returned, original) || len(owner.permission.stock[0].tokens) != 0 {
		t.Fatal("stock transfer wrong")
	}
	raw, err := os.ReadFile(filepath.Join(endpoint.closedTokenRoot, "attempts"))
	if err != nil || len(raw) != textTokenJournalHeader+textTokenAttemptSize || bytes.Contains(raw, original) {
		t.Fatal("missing durable receipt or leaked token")
	}
	receipt, err := decodeTextTokenAttempt(raw[textTokenJournalHeader:])
	if err != nil || receipt.attempt != hello.ChannelNonce || receipt.receiver != hello.RecipientNodeID {
		t.Fatal("receipt lost attempt binding")
	}
	hello.ChannelNonce[0]++
	if token, err := owner.presentTextToken(selection, hello, 2); err == nil || len(token) != 0 {
		t.Fatal("stock replayed")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openTextTokenJournal(endpoint.closedTokenRoot, endpoint.network, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.mark(original, receipt); err == nil {
		t.Fatal("context restart revived spent token")
	}
}
