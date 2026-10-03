//go:build linux

package issuer

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	admissionissuer "github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	admissiongrammar "github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// lateIssuerChildListener serves exactly one delayed Node child and then
// fails the accept loop, so the terminal cause reaches the late owner while
// the accepted child still borrows both roots.
type lateIssuerChildListener struct {
	ready      chan struct{}
	accepted   chan struct{}
	connection net.Conn
	failure    error
	served     bool
}

func (listener *lateIssuerChildListener) Accept(ctx context.Context, _ time.Duration) (carrier.ClosedSharedCarrier, error) {
	if !listener.served {
		select {
		case <-listener.ready:
		case <-ctx.Done():
			return carrier.ClosedSharedCarrier{}, ctx.Err()
		}
		listener.served = true
		close(listener.accepted)
		return carrier.ClosedSharedCarrier{Kind: carrier.ClosedSharedNode, Connection: listener.connection}, nil
	}
	return carrier.ClosedSharedCarrier{}, listener.failure
}

func (*lateIssuerChildListener) Close() error { return nil }

func TestClosedIssuerLateCloseRetainsRootsUntilDelayedChildJoins(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	certificate, public := nodeCertificate(t, 212, "closed-issuer-late-close")
	network, issuerID := [32]byte{51}, [32]byte{52}
	generation, digest := sha256.Sum256([]byte("late close generation")), sha256.Sum256([]byte("late close digest"))
	root := closedIssuerFixtureRoot(t)
	receipt, err := admissionissuer.InitializeClosedIssuerRoot(admissionissuer.ClosedIssuerRootConfig{Root: root, NetworkID: network, NodeID: issuerID,
		IdentityKey: certificate.PrivateKey.(ed25519.PrivateKey), NotBefore: now.Truncate(time.Hour), NotAfter: now.Truncate(time.Hour).Add(time.Hour), Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	issuerProfile, err := admissiongrammar.DecodeClosedIssuerProfile(receipt.Profile, ed25519.PublicKey(public[:]))
	if err != nil {
		t.Fatal(err)
	}
	profile := state.ClosedProfileView{NetworkID: network, StateGeneration: generation, StateDigest: digest, Digest: sha256.Sum256([]byte("late close profile")),
		IssuanceAuthorityKey: [32]byte{53}, IssuerNodeID: issuerID, IssuerDutyGeneration: 9, Epoch: 5,
		NotBefore: now.Truncate(time.Hour), NotAfter: now.Truncate(time.Hour).Add(time.Hour), TokenKeyCount: uint8(len(issuerProfile.Keys))}
	for index, key := range issuerProfile.Keys {
		profile.TokenKeys[index].WindowStart, profile.TokenKeys[index].Class = key.WindowStart, uint8(key.Class)
		copy(profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	issuerConfig := admissionissuer.ClosedTokenIssuerConfig{Root: root, NetworkID: network,
		CurrentProfile: func() (state.ClosedProfileView, bool) { return profile, true }, Clock: time.Now}
	issuer, err := admissionissuer.OpenClosedTokenIssuer(issuerConfig)
	if err != nil {
		t.Fatal(err)
	}
	admissionRoot := t.TempDir()
	binding := spending.Binding{NetworkID: network, ProfileDigest: profile.Digest, ReceiverNodeID: issuerID, ReceiverDutyGeneration: profile.IssuerDutyGeneration}
	spends, err := spending.Open(admissionRoot, binding)
	if err != nil {
		t.Fatal(err)
	}
	acceptErr := errors.New("issuer accept loop failed")
	local, peer := net.Pipe()
	handlerGate := make(chan struct{})
	var releaseGate sync.Once
	release := func() { releaseGate.Do(func() { close(handlerGate) }) }
	handler := admissionissuer.ClosedNodeBootstrapHandler(func(context.Context, carrier.ClosedSharedCarrier, func(context.Context, io.ReadWriter, [32]byte, ardp.Hello) error) {
		<-handlerGate
	})
	shared := &lateIssuerChildListener{ready: make(chan struct{}), accepted: make(chan struct{}), connection: local, failure: acceptErr}
	listener, err := admissionissuer.StartClosedTokenListener(context.Background(), admissionissuer.ClosedTokenListenerConfig{Issuer: issuer,
		SharedListener: shared, NodeHandler: handler, ConnectionLimit: 1, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	server := &closedIssuerServer{listener: listener, spends: spends, issuer: issuer, releases: &releaseErrors{},
		done: make(chan error, 1), drained: make(chan struct{})}
	t.Cleanup(func() {
		_ = server.listener.Stop()
		release()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.drain(ctx, time.Second)
		_ = spends.Close()
		_ = issuer.Close()
		_ = peer.Close()
	})
	go server.run()

	close(shared.ready)
	select {
	case <-shared.accepted:
	case <-time.After(time.Second):
		t.Fatal("shared listener did not accept the delayed issuer child")
	}
	// The terminal cause is forwarded to Node supervision while the delayed
	// child still holds both root leases.
	select {
	case cause := <-server.done:
		if !errors.Is(cause, acceptErr) {
			t.Fatalf("forwarded issuer terminal cause = %v", cause)
		}
	case <-time.After(time.Second):
		t.Fatal("late issuer owner did not forward the terminal cause")
	}
	if err := server.drain(t.Context(), 50*time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Drain completed while the accepted child still borrowed the roots: %v", err)
	}
	if reopened, err := admissionissuer.OpenClosedTokenIssuer(issuerConfig); err == nil {
		_ = reopened.Close()
		t.Fatal("unjoined issuer child lost its key root lease")
	}
	if replacement, err := spending.Open(admissionRoot, binding); err == nil {
		_ = replacement.Close()
		t.Fatal("unjoined issuer child lost its spend root lease")
	}
	// A lost journal file makes the eventual spend-root close fail; the late
	// owner must retain that distinct root-close error in its final result.
	retainedJournal := filepath.Join(t.TempDir(), "closed-token-spends")
	if err := os.Rename(filepath.Join(admissionRoot, "closed-token-spends"), retainedJournal); err != nil {
		t.Fatal(err)
	}
	if err := spends.Spend(bytes.Repeat([]byte{7}, 354), time.Now().UTC().Truncate(time.Hour), time.Now().UTC()); err == nil {
		t.Fatal("removed spend journal still accepted a durable write")
	}
	release()
	select {
	case <-server.drained:
	case <-time.After(time.Second):
		t.Fatal("delayed issuer child did not join and close the roots")
	}
	joined, cancelJoined := context.WithTimeout(t.Context(), time.Second)
	defer cancelJoined()
	for range 2 {
		err := server.drain(joined, time.Second)
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("joined issuer cleanup lost its spend-root close failure: %v", err)
		}
		if errors.Is(err, acceptErr) {
			t.Fatalf("joined issuer cleanup duplicated the forwarded terminal cause: %v", err)
		}
	}
	if reopened, err := spending.Open(admissionRoot, binding); err == nil {
		_ = reopened.Close()
		t.Fatal("joined shutdown recreated its missing spend journal")
	}
	// Restore the exact original journal to distinguish lease release from
	// accepting a damaged root. This fixture action is not a recovery route.
	if err := os.Rename(retainedJournal, filepath.Join(admissionRoot, "closed-token-spends")); err != nil {
		t.Fatal(err)
	}
	reopenedSpends, err := spending.Open(admissionRoot, binding)
	if err != nil {
		t.Fatalf("joined issuer shutdown retained the spend root: %v", err)
	}
	if err := reopenedSpends.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedIssuer, err := admissionissuer.OpenClosedTokenIssuer(issuerConfig)
	if err != nil {
		t.Fatalf("joined issuer shutdown retained the key root: %v", err)
	}
	if err := reopenedIssuer.Close(); err != nil {
		t.Fatal(err)
	}
}
