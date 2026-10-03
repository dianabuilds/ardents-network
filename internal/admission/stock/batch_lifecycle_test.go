//go:build linux

package stock

import (
	"bytes"
	"context"
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/client"
)

func TestPendingBatchOwnsExactRetryAndCallerIntent(t *testing.T) {
	owner, host, hello := issuedStockFixture(t)
	permission := owner.permission
	challenge := admissiontoken.ClosedTokenContext{NetworkID: host.profile.NetworkID, ProfileDigest: host.profile.Digest,
		IssuerNodeID: host.profile.IssuerNodeID, ReceiverNodeID: hello.RecipientNodeID,
		ReceiverDutyGeneration: hello.RecipientDutyGeneration, Class: 2, WindowStart: permission.Grant().NotBefore}
	intent := []admissiontoken.ClosedTokenContext{challenge, challenge}
	selection := client.ClosedBootstrapSelection{ProfileDigest: host.profile.Digest}
	prepared, err := permission.reserveBatch(host.profile, host.now, intent, selection, false, nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := prepared.Pending.Request()
	defer clear(original)
	reserved, batches := permission.reserved, permission.batches
	intent[0].ReceiverNodeID[0] ^= 1
	if prepared.Challenges[0] != challenge {
		t.Fatal("caller changed retained challenge through its input slice")
	}
	if _, err := permission.reserveBatch(host.profile, host.now, intent, selection, false, nil, false, nil); err == nil {
		t.Fatal("changed receiver replaced retained blind batch")
	}
	intent[0] = challenge
	changedSelection := selection
	changedSelection.ProfileDigest[0] ^= 1
	if _, err := permission.reserveBatch(host.profile, host.now, intent, changedSelection, false, nil, false, nil); err == nil {
		t.Fatal("changed selection replaced retained blind batch")
	}
	if _, err := permission.reserveBatch(host.profile, host.now, intent, selection, true, nil, false, nil); err == nil {
		t.Fatal("refill replaced ordinary retained batch")
	}
	retried, err := permission.reserveBatch(host.profile, host.now, intent, selection, false, nil, false, nil)
	if err != nil || retried != prepared || !bytes.Equal(original, retried.Pending.Request()) || permission.reserved != reserved || permission.batches != batches {
		t.Fatal("exact retry changed request, blinding owner, selection or debit")
	}
	permission.discardPendingBatch(prepared)
	if permission.HasPending() || len(prepared.Pending.Request()) != 0 || permission.reserved != reserved || permission.batches != batches {
		t.Fatal("discard failed to erase pending state or refunded reservation")
	}
}

func TestRevocationErasesPendingSecretsOnlyAfterCompletion(t *testing.T) {
	owner, host, hello := issuedStockFixture(t)
	permission := owner.permission
	challenge := admissiontoken.ClosedTokenContext{NetworkID: host.profile.NetworkID, ProfileDigest: host.profile.Digest,
		IssuerNodeID: host.profile.IssuerNodeID, ReceiverNodeID: hello.RecipientNodeID,
		ReceiverDutyGeneration: hello.RecipientDutyGeneration, Class: 2, WindowStart: permission.Grant().NotBefore}
	prepared, err := permission.reserveBatch(host.profile, host.now, []admissiontoken.ClosedTokenContext{challenge},
		client.ClosedBootstrapSelection{}, false, nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Isolate the private terminal ordering. Endpoint tests separately cancel
	// and join a real delayed network exchange through the public operation.
	operation := newOperation(owner, permission, host.profile, prepared, false)
	owner.issuance = operation
	holder := permission.holder
	owner.ClearPermissionLocked()
	detached := !owner.PermissionLocked().Present() && owner.BusyLocked() && len(prepared.Pending.Request()) != 0 && !bytes.Equal(holder, make([]byte, len(holder)))
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	completeErr := operation.complete(canceled, client.ClosedIssuanceExchangeResult{}, context.Canceled)
	operation.Join()
	if !detached || completeErr == nil || owner.BusyLocked() || len(prepared.Pending.Request()) != 0 || !bytes.Equal(holder, make([]byte, len(holder))) {
		t.Fatal("revocation released or erased pending ownership before terminal completion, or retained secrets afterward")
	}
}
