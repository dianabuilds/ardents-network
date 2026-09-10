//go:build linux

package state_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// Custody provisioning and Node duties run through actual commands. The exchange uses
// real State and Route, but is not the ordinary Endpoint command.
func prepareClosedProcessExchange(t *testing.T, network, issuer [32]byte, _ time.Time) ([32]byte, func(state.Config, bool)) {
	t.Helper()
	authority := createClosedCommandAuthority(t, network)
	var pending *credential.PendingClosedTokenBatch
	var retained []byte
	var challenge credential.ClosedTokenContext
	t.Cleanup(func() {
		if pending != nil {
			pending.Discard()
		}
		clear(retained)
	})
	return authority.Public, func(config state.Config, expectUnavailable bool) {
		t.Helper()
		config.Now, config.ClockObservation = time.Now().UTC(), time.Now().UTC()
		owner, err := state.Open(config)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := owner.Close(); err != nil {
				t.Error(err)
			}
		}()
		view, err := owner.CurrentClosedRoute()
		if err != nil {
			t.Fatal(err)
		}
		if pending == nil {
			challenge = credential.ClosedTokenContext{NetworkID: network, ProfileDigest: view.Profile.Digest, ReceiverNodeID: [32]byte{1},
				IssuerNodeID: issuer, ReceiverDutyGeneration: 1, Class: 1, WindowStart: config.Now.Truncate(time.Hour)}
			pending = closedProcessBatch(t, authority, view.Profile, challenge, config.Now)
			retained = pending.Request()
		} else if challenge.ProfileDigest != view.Profile.Digest || !bytes.Equal(pending.Request(), retained) {
			t.Fatal("recovery replaced the retained batch or State profile")
		}
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		response, exchangeErr := route.ExchangeClosedBootstrap(ctx, owner, route.ClosedBootstrapSelection{
			ProfileDigest: view.Profile.Digest, EntryNodeID: [32]byte{1}, InteriorNodeID: [32]byte{3}}, retained)
		defer clear(response.Body)
		if expectUnavailable {
			if exchangeErr == nil || ctx.Err() != nil || errors.Is(exchangeErr, route.ErrClosedBootstrapCleanup) ||
				response.Nonce != [32]byte{} || len(response.Body) != 0 || !bytes.Equal(pending.Request(), retained) {
				t.Fatalf("issuer loss did not cleanly refuse while preserving the exact batch: %v", exchangeErr)
			}
			t.Logf("observed bounded refusal with exact pending batch retained: %v", exchangeErr)
			return
		}
		if exchangeErr != nil {
			t.Fatalf("issuance through real Node processes: %v", exchangeErr)
		}
		tokens, err := pending.FinalizeTerminalOperation(response.Nonce, response.Body)
		pending = nil
		clear(retained)
		retained = nil
		if err != nil || len(tokens) != 1 {
			t.Fatalf("verify issued blind token: %v / %d", err, len(tokens))
		}
		defer clear(tokens[0])
		var spki []byte
		for _, key := range view.Profile.TokenKeys[:view.Profile.TokenKeyCount] {
			if key.Class == 1 && key.WindowStart == challenge.WindowStart {
				spki = key.SPKI[:]
			}
		}
		if err := credential.VerifyClosedToken(challenge, spki, tokens[0]); err != nil {
			t.Fatal(err)
		}
		t.Log("verified issued token from command-owned Entry, Interior and issuer")
	}
}

func closedProcessBatch(t *testing.T, authority closedCommandAuthority, profile state.ClosedProfileView,
	challenge credential.ClosedTokenContext, now time.Time) *credential.PendingClosedTokenBatch {
	t.Helper()
	request, holder, err := credential.PreparePermissionRequest(authority.Public, challenge.NetworkID, challenge.IssuerNodeID,
		profile.IssuerDutyGeneration, credential.AllocationUser, challenge.WindowStart, [3]uint32{1, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	defer clear(holder)
	raw, err := credential.EncodePermissionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	permission, err := credential.DecodePermission(authority.issue(t, raw))
	if err != nil {
		t.Fatal(err)
	}
	batch, err := credential.PrepareClosedTokenBatch(credential.ClosedTokenBatchConfig{Profile: profile,
		Contexts: []credential.ClosedTokenContext{challenge}, Permission: permission, HolderKey: holder, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	return batch
}
