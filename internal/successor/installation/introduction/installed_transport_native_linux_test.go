//go:build linux && introduction_installed_native

package introduction_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
)

type introductionNativeChannel struct {
	parent   *framing.Session
	lane     *framing.Lane
	conn     net.Conn
	check    func() error
	deadline time.Time
	once     sync.Once
	result   error
}

func (c *introductionNativeChannel) close() error {
	c.once.Do(func() {
		if c.conn != nil {
			c.result = errors.Join(c.result, c.conn.Close())
		}
		if c.lane != nil {
			c.lane.Finish()
		}
		if c.parent != nil {
			c.result = errors.Join(c.result, c.parent.Close())
		}
	})
	return c.result
}

// After the exact operation result, join TLS and the actual clean lower terminal.
// EOF alone is not receiving completion and cannot release the parent early.
func (c *introductionNativeChannel) finish(ctx context.Context) error {
	writer, ok := c.conn.(interface{ CloseWrite() error })
	if !ok {
		return errors.New("original TLS half-close absent")
	}
	if err := writer.CloseWrite(); err != nil {
		return err
	}
	var extra [1]byte
	if n, err := c.conn.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		return errors.Join(errors.New("original TLS termination absent"), err)
	}
	if n, err := c.lane.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		return errors.Join(errors.New("original lower termination absent"), err)
	}
	cleanup, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	interrupted := make(chan struct{})
	var interruptErr error
	stop := context.AfterFunc(cleanup, func() { defer close(interrupted); interruptErr = c.lane.InterruptIO() })
	err := c.lane.WaitPeerRetirement(cleanup)
	if !stop() {
		<-interrupted
	}
	return errors.Join(err, interruptErr, c.check(), c.close())
}

// Each adjacent channel has its own real finite right and authenticated peer.
// This protocol harness is not a Source prefix, Publisher or Service consumer.
func openIntroductionNativeChannel(t *testing.T, prepared introductionPreparedStock, certificate tls.Certificate, purpose uint8) *introductionNativeChannel {
	t.Helper()
	peer := prepared.registrationPeer
	if purpose == uint8(ardp.PurposeSubmission) {
		peer = prepared.submissionPeer
	}
	class, err := role.AdmissionClass(ardp.Purpose(purpose))
	if err != nil || purpose != uint8(ardp.PurposeIntroduction) && purpose != uint8(ardp.PurposeSubmission) {
		t.Fatal("exact Introduction purpose required", err)
	}
	deadline := time.Now().UTC().Add(25 * time.Second).Truncate(time.Second)
	if purpose == uint8(ardp.PurposeIntroduction) {
		deadline = time.Now().UTC().Add(90 * time.Second).Truncate(time.Second)
	}
	hourEnd := time.Now().UTC().Truncate(time.Hour).Add(time.Hour)
	if deadline.After(hourEnd) {
		deadline = hourEnd
	}
	if deadline.After(prepared.facts.NotAfter) {
		deadline = prepared.facts.NotAfter
	}
	var nonce, outerNonce [32]byte
	for _, p := range [][]byte{nonce[:], outerNonce[:]} {
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
		currentPeer, err := view.Member(peer.NodeID, view.ObservedAt())
		if err != nil || currentPeer.RecordDigest != peer.RecordDigest || currentPeer.DutyGeneration != peer.DutyGeneration || currentPeer.RoleDomain != peer.RoleDomain || currentPeer.Subrole != peer.Subrole {
			return errors.Join(errors.New("original peer changed"), err)
		}
		member, err := view.Member(prepared.receiver.NodeID, view.ObservedAt())
		if err != nil || member.PublicKey != prepared.publicKey || member.DutyGeneration != prepared.receiver.DutyGeneration || member.RoleDomain != 4 || member.Subrole != 3 || role.Conflicting(member, peer) {
			return errors.Join(errors.New("original Introduction changed"), err)
		}
		return nil
	}
	if private, ok := certificate.PrivateKey.(ed25519.PrivateKey); !ok || !bytes.Equal(private.Public().(ed25519.PublicKey), peer.PublicKey[:]) {
		t.Fatal("canonical adjacent key differs")
	}
	token, err := prepared.holder.Take(t.Context(), stock.Presentation{NetworkID: prepared.facts.NetworkID, StateGeneration: prepared.facts.StateGeneration, StateDigest: prepared.facts.StateDigest, ProfileDigest: prepared.facts.Digest, RecipientNodeID: prepared.receiver.NodeID, RecipientDutyGeneration: prepared.receiver.DutyGeneration, ChannelNonce: nonce, Deadline: deadline}, uint8(class))
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
	var retained *introductionNativeChannel
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
	retained = &introductionNativeChannel{parent: parent, check: check, deadline: deadline}
	t.Cleanup(func() {
		if err := retained.close(); err != nil {
			t.Error("original channel cleanup", err)
		}
	})
	opened, err := ardp.EncodeNodeOpen(ardp.NodeOpen{Recipient: ardp.Open{RecipientNodeID: prepared.receiver.NodeID, RecipientDutyGeneration: prepared.receiver.DutyGeneration, Purpose: uint8(ardp.Purpose(purpose)), Deadline: deadline}, Restriction: ardp.OrdinaryChild})
	if err != nil {
		t.Fatal(err)
	}
	lane, err := parent.Open(t.Context(), t.Context(), opened)
	if err != nil {
		t.Fatal("actual Introduction lower lane", err)
	}
	retained.lane = lane
	secured, err := roletls.OpenRole(t.Context(), lane, prepared.publicKey, deadline)
	if err != nil {
		t.Fatal("actual nested Introduction TLS", err)
	}
	roleBegan := time.Now()
	conn := transport.Retain(secured)
	retained.conn = conn
	if err := conn.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	hello.Purpose = ardp.Purpose(purpose)
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
	body := append([]byte{byte(class)}, token...)
	send(ardp.Frame{Kind: ardp.KindAdmit, Body: body})
	clear(body)
	clear(token)
	ack, err := ardp.ReadFrame(conn)
	if err != nil {
		t.Fatal("actual class-bound admission", err, "role elapsed", time.Since(roleBegan))
	}
	status, credit, err := ardp.DecodeAcceptFrame(ack)
	if err != nil || status != 0 || credit != 64<<10 {
		t.Fatal("actual class-bound acceptance", status, credit, err)
	}
	return retained
}
