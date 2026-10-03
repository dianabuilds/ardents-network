//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"

	"github.com/dianabuilds/ardents-network/internal/admission/attempts"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// A real Custody permission and actual issuer produce the stock fixture.
// The qualified-worker/State and in-flight-opening seams are explicit fixtures;
// this test verifies the Endpoint's real stock-to-journal consumer.
func TestTextTokenPresentationBurnsStockBeforeReturningBytes(t *testing.T) {
	endpoint, owner, selection, profile, hello := tokenPresentationFixture(t)
	flightContext, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	close(done)
	opening := &operationFlight{owner: owner, context: flightContext, cancelOperation: cancel, done: done}
	if !owner.source.ReserveOpeningLocked(opening) {
		t.Fatal("Source lifecycle refused the planted opening")
	}
	returned, err := opening.presentToken(selection, hello, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(returned)
	original := bytes.Clone(returned)
	defer clear(original)
	if len(returned) != 354 || owner.tokens.PermissionLocked().StockCountForDuty(profile.Digest, hello.RecipientNodeID, hello.RecipientDutyGeneration, 2) != 0 {
		t.Fatal("stock transfer wrong")
	}
	raw, err := os.ReadFile(filepath.Join(endpoint.closedTokenRoot, "attempts"))
	if err != nil || len(raw) != tokenReceiptHeaderSize+tokenReceiptSize || bytes.Contains(raw, original) {
		t.Fatal("missing durable receipt or leaked token")
	}
	receipts := parseTokenReceipts(t, raw, endpoint.network)
	if len(receipts) != 1 || receipts[0].attempt != hello.ChannelNonce || receipts[0].receiver != hello.RecipientNodeID {
		t.Fatal("receipt lost attempt binding")
	}
	hello.ChannelNonce[0]++
	if token, err := opening.presentToken(selection, hello, 2); err == nil || len(token) != 0 {
		t.Fatal("stock replayed")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := attempts.Open(endpoint.closedTokenRoot, endpoint.network, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Mark(original, journalAttemptFromReceipt(receipts[0])); err == nil {
		t.Fatal("context restart revived spent token")
	}
}

func TestTextTokenCancellationAfterDurableMarkRetainsBurn(t *testing.T) {
	endpoint, owner, _, profile, hello := tokenPresentationFixture(t)
	attempt, cancel := context.WithCancel(t.Context())
	cancel()
	owner.mu.Lock()
	returned, err := owner.tokens.TakeTokenLocked(profile, time.Now().UTC(), hello, 2, attempt)
	remaining := owner.tokens.PermissionLocked().StockCountForDuty(profile.Digest, hello.RecipientNodeID, hello.RecipientDutyGeneration, 2)
	owner.mu.Unlock()
	if err == nil || len(returned) != 0 || remaining != 0 || stock.TransferFailureStage(err) != "owner" {
		t.Fatalf("cancelled durable spend returned token or wrong result: bytes=%d remaining=%d stage=%s err=%v",
			len(returned), remaining, stock.TransferFailureStage(err), err)
	}
	receipts := readTokenReceipts(t, endpoint.closedTokenRoot, endpoint.network)
	if len(receipts) != 1 || receipts[0].attempt != hello.ChannelNonce {
		t.Fatal("cancelled spend lost its durable attempt receipt")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := attempts.Open(endpoint.closedTokenRoot, endpoint.network, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if after := readTokenReceipts(t, endpoint.closedTokenRoot, endpoint.network); len(after) != 1 || after[0] != receipts[0] {
		t.Fatal("restart lost or changed the canceled presentation receipt")
	}
}

func tokenPresentationFixture(t *testing.T) (*endpoint, *dutyContext, client.ClosedBootstrapSelection,
	state.ClosedProfileView, ardp.Hello) {
	t.Helper()
	endpoint, owner, source := startRoleNetwork(t, roleNetworkFixture{carrier: carrier.ClosedCarrierTCP})
	selection := selectSource(t, owner)
	profile := source.view.Profile
	duty := uint64(0)
	for _, node := range source.view.Nodes[:source.view.NodeCount] {
		if node.NodeID == selection.EntryNodeID {
			duty = node.DutyGeneration
		}
	}
	if err := owner.issueTokens(t.Context(), [][32]byte{selection.EntryNodeID}, 2); err != nil {
		t.Fatal(err)
	}
	hello := ardp.Hello{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest,
		ProfileDigest: profile.Digest, RecipientNodeID: selection.EntryNodeID, RecipientDutyGeneration: duty,
		Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{242}, Deadline: profile.NotAfter}
	return endpoint, owner, selection, profile, hello
}
