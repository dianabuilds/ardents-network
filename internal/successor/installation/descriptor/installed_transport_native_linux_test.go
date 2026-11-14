//go:build linux && descriptor_installed_native

package descriptor_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
)

// Every operation has its own spent Control right, original authenticated TLS
// channel, result nonce and complete inner/lower termination. No Source prefix,
// live Publisher or full Service chain is represented by this adjacent peer.
func descriptorNativeOperation(t *testing.T, prepared descriptorPreparedStock, certificate tls.Certificate, request ardp.DescriptorRequest) (uint8, []byte) {
	t.Helper()
	began := time.Now()
	phase := func(name string) {
		t.Helper()
		t.Logf("Descriptor physical %s at %.3fs", name, time.Since(began).Seconds())
	}
	deadline := time.Now().UTC().Add(30 * time.Second).Truncate(time.Second)
	hourEnd := time.Now().UTC().Truncate(time.Hour).Add(time.Hour)
	if deadline.After(hourEnd) {
		deadline = hourEnd
	}
	if deadline.After(prepared.facts.NotAfter) {
		deadline = prepared.facts.NotAfter
	}
	var nonce, outerNonce [32]byte
	for _, p := range [][]byte{nonce[:], outerNonce[:], request.Nonce[:]} {
		if _, err := rand.Read(p); err != nil {
			t.Fatal(err)
		}
	}
	check := func() error {
		view, err := prepared.current()
		if err != nil {
			return err
		}
		profile := view.Profile()
		if profile.Digest != prepared.facts.Digest || profile.Generation != prepared.facts.StateGeneration || profile.EpochDigest != prepared.facts.StateDigest {
			return errors.New("original authenticated profile changed")
		}
		peer, err := view.Member(prepared.peer.NodeID, view.ObservedAt())
		if err != nil || peer.RecordDigest != prepared.peer.RecordDigest || peer.DutyGeneration != prepared.peer.DutyGeneration {
			return errors.Join(errors.New("original peer changed"), err)
		}
		member, err := view.Member(prepared.receiver.NodeID, view.ObservedAt())
		if err != nil || member.PublicKey != prepared.publicKey || member.DutyGeneration != prepared.receiver.DutyGeneration || member.RoleDomain != 2 || member.Subrole != 5 || role.Conflicting(member, peer) {
			return errors.Join(errors.New("original Descriptor changed"), err)
		}
		return nil
	}
	if private, ok := certificate.PrivateKey.(ed25519.PrivateKey); !ok || !bytes.Equal(private.Public().(ed25519.PublicKey), prepared.peer.PublicKey[:]) {
		t.Fatal("canonical adjacent key differs")
	}
	token, err := prepared.holder.Take(t.Context(), stock.Presentation{NetworkID: prepared.facts.NetworkID, StateGeneration: prepared.facts.StateGeneration, StateDigest: prepared.facts.StateDigest, ProfileDigest: prepared.facts.Digest, RecipientNodeID: prepared.receiver.NodeID, RecipientDutyGeneration: prepared.receiver.DutyGeneration, ChannelNonce: nonce, Deadline: deadline}, uint8(admission.ControlClass))
	if err != nil {
		t.Fatal("durable original presentation", err)
	}
	defer clear(token)
	if err := check(); err != nil {
		t.Fatal(err)
	}
	physical, err := roletls.OpenNode(t.Context(), transport.ClosedNodeCarrierRequest{CarrierProfile: transport.ClosedCarrierTCP, Endpoint: prepared.endpoint, Certificate: certificate, ExpectedPeerKey: prepared.publicKey, Deadline: deadline})
	if err != nil {
		t.Fatal("actual Node TLS", err)
	}
	transferred := false
	defer func() {
		if !transferred {
			if err := physical.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	outer, ok := physical.(net.Conn)
	if !ok {
		t.Fatal("actual Node Carrier lacks net.Conn")
	}
	if err := outer.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	hello := ardp.Hello{NetworkID: prepared.facts.NetworkID, StateGeneration: prepared.facts.StateGeneration, StateDigest: prepared.facts.StateDigest, ProfileDigest: prepared.facts.Digest, RecipientNodeID: prepared.receiver.NodeID, RecipientDutyGeneration: prepared.receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: outerNonce, Deadline: deadline}
	if err := framing.SendHello(outer, hello); err != nil {
		t.Fatal(err)
	}
	if err := framing.Accepted(outer); err != nil {
		t.Fatal("actual outer acceptance", err)
	}
	parent := framing.New(t.Context(), outer, deadline, 32<<20, check, false, framing.NewBudget(2<<20), nil)
	transferred = true
	defer func() {
		if err := parent.Close(); err != nil {
			t.Error("original outer join", err, "physical failure", parent.PhysicalFailure(), "elapsed", time.Since(began))
		}
	}()
	opened, err := ardp.EncodeNodeOpen(ardp.NodeOpen{Recipient: ardp.Open{RecipientNodeID: prepared.receiver.NodeID, RecipientDutyGeneration: prepared.receiver.DutyGeneration, Purpose: uint8(ardp.PurposeReachability), Deadline: deadline}, Restriction: ardp.OrdinaryChild})
	if err != nil {
		t.Fatal(err)
	}
	lane, err := parent.Open(t.Context(), t.Context(), opened)
	if err != nil {
		t.Fatal("actual Descriptor lower lane", err)
	}
	defer lane.Finish()
	secured, err := roletls.OpenRole(t.Context(), lane, prepared.publicKey, deadline)
	if err != nil {
		t.Fatal("actual nested Descriptor TLS", err)
	}
	roleBegan := time.Now()
	conn := transport.Retain(secured)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Error("original role TLS Close", err)
		}
	}()
	if err := conn.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	hello.Purpose = ardp.PurposeReachability
	hello.ChannelNonce = nonce
	raw, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	send := func(f ardp.Frame) {
		t.Helper()
		if err := ardp.WriteFrame(conn, f); err != nil {
			t.Fatal("original frame write", err)
		}
	}
	send(ardp.Frame{Kind: ardp.KindHello, Body: raw})
	body := append([]byte{1}, token...)
	send(ardp.Frame{Kind: ardp.KindAdmit, Body: body})
	clear(body)
	clear(token)
	ack, err := ardp.ReadFrame(conn)
	if err != nil {
		t.Fatal("actual Control admission", err, "role elapsed", time.Since(roleBegan))
	}
	status, credit, err := ardp.DecodeAcceptFrame(ack)
	if err != nil || status != 0 || credit != 65536 {
		t.Fatal("actual Control acceptance", status, credit, err)
	}
	raw, err = ardp.EncodeDescriptorRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	send(ardp.Frame{Kind: ardp.KindOperation, Body: raw})
	response, err := ardp.ReadFrame(conn)
	if err != nil || response.Kind != ardp.KindResult || response.Lane != 0 {
		t.Fatal("actual lane-zero RESULT", response.Kind, response.Lane, err)
	}
	status, payload, err := ardp.DecodeDescriptorResult(response.Body, request.Operation, request.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	result := bytes.Clone(payload)
	phase("RESULT verified")
	writer, ok := conn.(interface{ CloseWrite() error })
	if !ok {
		t.Fatal("TLS half-close unavailable")
	}
	if err := writer.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	var extra [1]byte
	if n, err := conn.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatal("original TLS peer termination", n, err)
	}
	if n, err := lane.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatal("original lower termination", n, err)
	}
	// A directional EOF does not prove the receiving handler's clean terminal
	// or join its outstanding output. Retain the actual parent until this exact
	// lower lane has observed CLOSE(0) and joined its original CREDIT writers.
	// Use the existing one-second cleanup horizon, never extend the operation.
	cleanup, cancelCleanup := context.WithTimeout(t.Context(), time.Second)
	interrupted := make(chan struct{})
	var interruptErr error
	stopCleanup := context.AfterFunc(cleanup, func() {
		defer close(interrupted)
		interruptErr = lane.InterruptIO()
	})
	terminalErr := lane.WaitPeerRetirement(cleanup)
	if !stopCleanup() {
		<-interrupted
	}
	cancelCleanup()
	if err := errors.Join(terminalErr, interruptErr); err != nil {
		t.Fatal("original lower peer terminal/join", err, "elapsed", time.Since(began))
	}
	phase("exact lower terminal and output joined")
	if err := check(); err != nil {
		t.Fatal(err)
	}
	return status, result
}

func runDescriptorNativeOperations(t *testing.T, prepared descriptorPreparedStock, certificate tls.Certificate, proof descriptorNativeProofs) {
	t.Helper()
	var operations atomic.Int64
	spends := func() []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(descriptorNativeParent, "runtime/spend/closed-token-spends"))
		if err != nil || len(raw) < 112 || string(raw[:8]) != "ARDSPN01" || (len(raw)-112)%41 != 0 {
			t.Fatal("independent receiving spend framing", err)
		}
		if !bytes.Equal(raw[8:40], prepared.facts.NetworkID[:]) || !bytes.Equal(raw[40:72], prepared.facts.Digest[:]) || !bytes.Equal(raw[72:104], prepared.receiver.NodeID[:]) || binary.BigEndian.Uint64(raw[104:112]) != prepared.receiver.DutyGeneration {
			t.Fatal("original receiving spend binding changed")
		}
		return raw
	}
	verifySpends := func(previous []byte, count int) {
		t.Helper()
		next := spends()
		if len(next) != len(previous)+count*41 || !bytes.Equal(next[:len(previous)], previous) {
			t.Fatal("one irreversible spend per operation required; earlier spends must remain")
		}
		seen := map[[32]byte]bool{}
		for offset := len(previous); offset < len(next); offset += 41 {
			var digest [32]byte
			copy(digest[:], next[offset:offset+32])
			if digest == ([32]byte{}) || seen[digest] || next[offset+40] != 1 {
				t.Fatal("distinct fully committed receiving spend required")
			}
			seen[digest] = true
		}
	}
	perform := func(name string, request ardp.DescriptorRequest, want uint8, wantProof []byte) {
		t.Helper()
		began := time.Now()
		status, payload := descriptorNativeOperation(t, prepared, certificate, request)
		operations.Add(1)
		if status != want || !bytes.Equal(payload, wantProof) {
			t.Fatal(name, "RESULT differs", status, want, len(payload), len(wantProof))
		}
		if len(payload) != 0 {
			if _, err := reachability.Verify(payload, request.Target, prepared.facts.NetworkID, prepared.facts.Digest, time.Now()); err != nil {
				t.Fatal("independent returned proof verification", err)
			}
		}
		t.Logf("%s: status=%d, exact public result, joined original channel (%.3fs)", name, status, time.Since(began).Seconds())
	}
	sequential := func(name string, request ardp.DescriptorRequest, want uint8, wantProof []byte) {
		t.Helper()
		previous := spends()
		perform(name, request, want, wantProof)
		verifySpends(previous, 1)
	}
	publish := func(raw []byte) ardp.DescriptorRequest {
		return ardp.DescriptorRequest{Operation: ardp.DescriptorPublish, Proof: raw}
	}
	lookup := func(target [32]byte) ardp.DescriptorRequest {
		return ardp.DescriptorRequest{Operation: ardp.DescriptorLookup, Target: target}
	}
	sequential("publish revision1 Store ACK", publish(proof.first), 0, nil)
	sequential("lookup exact independently selected Target", lookup(proof.target), 0, proof.first)
	bad := bytes.Clone(proof.first)
	bad[len(bad)-1] ^= 1
	absent := proof.target
	absent[0] ^= 1
	// These four requests cannot mutate Store and have independent rights,
	// nonces, deadlines and channels. Join all before checking retained bytes.
	previous := spends()
	tests := []struct {
		name    string
		request ardp.DescriptorRequest
		status  uint8
	}{
		{"invalid Instance signature", publish(bad), 1},
		{"absent Target no fallback", lookup(absent), 3},
		{"signed foreign profile refused", publish(proof.foreign), 1},
		{"future signed interval refused", publish(proof.future), 1},
	}
	// Leave the receiver's unchanged ten-second setup deadline room for
	// genuine serialized Admission and fresh native observations. This is a
	// harness concurrency bound, never a protocol retry or refunded right.
	for start := 0; start < len(tests); start += 2 {
		var refusals sync.WaitGroup
		for _, test := range tests[start:min(start+2, len(tests))] {
			refusals.Go(func() { perform(test.name, test.request, test.status, nil) })
		}
		refusals.Wait()
		if t.Failed() {
			t.FailNow()
		}
	}
	verifySpends(previous, 4)
	sequential("bad signature preserved Store", lookup(proof.target), 0, proof.first)
	sequential("same-revision conflict", publish(proof.conflict), 3, nil)
	sequential("conflicted lookup no fallback", lookup(proof.target), 3, nil)
	sequential("higher revision repairs conflict", publish(proof.second), 0, nil)
	sequential("lookup exact repaired revision2", lookup(proof.target), 0, proof.second)
	sequential("older revision refused", publish(proof.first), 3, nil)
	sequential("refusals preserved newer Store", lookup(proof.target), 0, proof.second)
	t.Logf("completed %d actual Control operations; no right refunded", operations.Load())
}
