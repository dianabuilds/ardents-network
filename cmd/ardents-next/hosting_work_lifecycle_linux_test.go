//go:build linux

package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

func TestCanceledSocketChildRetainsReservationUntilJoined(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	spki, err := issuerprofile.EncodeKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	window := now.Truncate(time.Hour)
	recipient := receiving.Receiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, DutyGeneration: 6}
	facts := admission.AuthorityFacts{NetworkID: recipient.NetworkID, StateGeneration: recipient.StateGeneration, StateDigest: recipient.StateDigest, Digest: recipient.ProfileDigest, IssuerNodeID: [32]byte{7}, IssuerDutyGeneration: 1, IssuanceAuthorityKey: [32]byte{9}, NotBefore: window, NotAfter: window.Add(time.Hour), TokenKeyCount: 3}
	for i := range 3 {
		public := spki
		if i != 1 {
			other, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				t.Fatal(err)
			}
			public, err = issuerprofile.EncodeKey(&other.PublicKey)
			if err != nil {
				t.Fatal(err)
			}
		}
		facts.TokenKeys[i] = admission.TokenKey{WindowStart: window, Class: uint8(i + 1)}
		copy(facts.TokenKeys[i].SPKI[:], public)
	}
	challenge := token.ClosedTokenContext{NetworkID: recipient.NetworkID, ProfileDigest: recipient.ProfileDigest, IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: recipient.NodeID, ReceiverDutyGeneration: recipient.DutyGeneration, Class: 2, WindowStart: window}
	_, input, _, err := token.ClosedTokenChallenge(challenge, spki, [32]byte{8})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha512.Sum384(input)
	signature, err := rsa.SignPSS(rand.Reader, key, crypto.SHA384, digest[:], &rsa.PSSOptions{SaltLength: crypto.SHA384.Size(), Hash: crypto.SHA384})
	if err != nil {
		t.Fatal(err)
	}
	raw := append(input, signature...)
	budgetRoot := filepath.Join(t.TempDir(), "budget")
	policy := hosting.Policy{Provider: "local test", Start: window, End: window.Add(2 * time.Hour), Unit: "MiB", Quantity: 100, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000}
	if err := hosting.Initialize(budgetRoot, policy); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(budgetRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	plan := hostingWorkFixture{Root: t.TempDir(), Budget: budgetRoot, Receiver: recipient, NotAfter: facts.NotAfter, Deadline: time.Now().Add(4 * time.Second), Token: raw, Bytes: 64 << 10, Observe: func() (admission.AuthorityFacts, time.Time, error) { return facts, time.Now().UTC(), nil }}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	childStarted := make(chan struct{})
	childStopped := make(chan struct{})
	allowJoin := make(chan struct{})
	transfer := func(ctx context.Context, count uint64, deadline time.Time, current func() error) error {
		return transferLocal(ctx, count, deadline, current, func(reader io.Reader, n int64) error {
			close(childStarted)
			<-ctx.Done()
			err := discardPayload(reader, n)
			close(childStopped)
			<-allowJoin
			return err
		})
	}
	done := make(chan error, 1)
	go func() { done <- runHostingFixtureWithTransfer(ctx, plan, transfer) }()
	select {
	case <-childStarted:
	case err := <-done:
		t.Fatal("failed before socket reader", err)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	cancel()
	<-childStopped
	view, observeErr := budget.Observe(t.Context())
	close(allowJoin)
	outcome := <-done
	if observeErr != nil || view.ReservedBytes == 0 {
		t.Fatal("unjoined socket child lost reservation", view, observeErr)
	}
	if !errors.Is(outcome, context.Canceled) {
		t.Fatal("lost cancellation", outcome)
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("joined socket child retained reservation", view, err)
	}
	plan.Deadline = time.Now().Add(4 * time.Second)
	if err := runHostingFixture(t.Context(), plan); err == nil {
		t.Fatal("spent token was refunded after I/O cancellation")
	}
}
