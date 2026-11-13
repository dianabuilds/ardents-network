//go:build linux && join_installed_native

package join_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"errors"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"net"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
)

// The driver owns two actual TLS clients and an independent byte oracle. This
// is one receiving JOIN role, not a complete protected Route or Service claim.
func runJoinInstalledPair(t *testing.T, prepared joinPreparedStock, certificate tls.Certificate) {
	t.Helper()
	began := time.Now()
	phase := func(name string) { t.Helper(); t.Logf("pair phase %s: %.3fs", name, time.Since(began).Seconds()) }
	phase("begin")
	deadline := time.Now().UTC().Add(30 * time.Second).Truncate(time.Second)
	hourEnd := time.Now().UTC().Truncate(time.Hour).Add(time.Hour)
	if deadline.After(hourEnd) {
		deadline = hourEnd
	}
	if deadline.After(prepared.facts.NotAfter) {
		deadline = prepared.facts.NotAfter
	}
	var secret, context [32]byte
	for _, p := range [][]byte{secret[:], context[:]} {
		if _, err := rand.Read(p); err != nil {
			t.Fatal(err)
		}
	}
	defer clear(secret[:])
	defer clear(context[:])
	var channels [2]net.Conn
	var parents [2]*framing.Session
	var lanes [2]*framing.Lane
	defer func() {
		for i, parent := range parents {
			if lanes[i] != nil {
				lanes[i].Finish()
			}
			if parent != nil {
				if err := parent.Close(); err != nil {
					t.Error("original outer session join", err)
				}
			}
		}
	}()
	check := func() error {
		view, err := prepared.current()
		if err != nil {
			return err
		}
		peer, err := view.Member(prepared.peer.NodeID, view.ObservedAt())
		if err != nil || peer.RecordDigest != prepared.peer.RecordDigest || peer.DutyGeneration != prepared.peer.DutyGeneration {
			return errors.Join(errors.New("original peer changed"), err)
		}
		member, err := view.Member(prepared.receiver.NodeID, view.ObservedAt())
		if err != nil || member.PublicKey != prepared.publicKey || member.DutyGeneration != prepared.receiver.DutyGeneration || role.Conflicting(member, peer) {
			return errors.Join(errors.New("original JOIN changed"), err)
		}
		return nil
	}
	if private, ok := certificate.PrivateKey.(ed25519.PrivateKey); !ok || !bytes.Equal(private.Public().(ed25519.PublicKey), prepared.peer.PublicKey[:]) {
		t.Fatal("canonical adjacent transport key differs")
	}

	var requests [2]ardp.JoinRequest
	defer func() {
		for _, conn := range channels {
			if conn != nil {
				if err := conn.Close(); err != nil {
					t.Error("original TLS Close", err)
				}
			}
		}
	}()
	send := func(conn net.Conn, f ardp.Frame) {
		t.Helper()
		if err := ardp.WriteFrame(conn, f); err != nil {
			t.Fatal("actual client frame write", err)
		}
	}
	receive := func(conn net.Conn, kind uint8) ardp.Frame {
		t.Helper()
		frame, err := ardp.ReadFrame(conn)
		if err != nil && kind == ardp.KindEOF {
			raw, cut := exec.Command("systemctl", "kill", "--signal=SIGQUIT", "--kill-whom=main", "ardents-join.service").CombinedOutput()
			t.Log("failure-only original Node goroutine capture", cut, string(raw))
		}
		if err != nil || frame.Kind != kind || frame.Lane != 1 {
			t.Fatal("actual client frame read", kind, frame.Kind, frame.Lane, err)
		}
		return frame
	}
	var setup sync.WaitGroup
	for side := range channels {
		setup.Go(func() {
			var nonce, requestNonce, outerNonce [32]byte
			for _, p := range [][]byte{nonce[:], requestNonce[:], outerNonce[:]} {
				if _, err := rand.Read(p); err != nil {
					t.Fatal(err)
				}
			}
			// Actual Stock durably marks presentation before the first network byte.
			token, err := prepared.holder.Take(t.Context(), stock.Presentation{NetworkID: prepared.facts.NetworkID, StateGeneration: prepared.facts.StateGeneration, StateDigest: prepared.facts.StateDigest, ProfileDigest: prepared.facts.Digest, RecipientNodeID: prepared.receiver.NodeID, RecipientDutyGeneration: prepared.receiver.DutyGeneration, ChannelNonce: nonce, Deadline: deadline}, uint8(admission.ForwardClass))
			if err != nil {
				t.Fatal(err)
			}
			if err := check(); err != nil {
				t.Fatal(err)
			}
			physical, err := roletls.OpenNode(t.Context(), transport.ClosedNodeCarrierRequest{CarrierProfile: transport.ClosedCarrierTCP, Endpoint: prepared.endpoint, Certificate: certificate, ExpectedPeerKey: prepared.publicKey, Deadline: deadline})
			if err != nil {
				t.Fatal("actual authenticated adjacent Node TLS", err)
			}
			phase("Node TLS established")
			transferred := false
			defer func() {
				if !transferred {
					if err := physical.Close(); err != nil {
						t.Error("rejected original Node Carrier Close", err)
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
			h := ardp.Hello{NetworkID: prepared.facts.NetworkID, StateGeneration: prepared.facts.StateGeneration, StateDigest: prepared.facts.StateDigest, ProfileDigest: prepared.facts.Digest, RecipientNodeID: prepared.receiver.NodeID, RecipientDutyGeneration: prepared.receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: outerNonce, Deadline: deadline}
			if err := framing.SendHello(outer, h); err != nil {
				t.Fatal(err)
			}
			if err := framing.Accepted(outer); err != nil {
				t.Fatal("actual adjacent carrier acceptance", err)
			}
			if err := check(); err != nil {
				t.Fatal(err)
			}
			phase("Node HELLO accepted")
			parents[side] = framing.New(t.Context(), outer, deadline, 32<<20, check, false, framing.NewBudget(2<<20), nil)
			transferred = true
			opened, err := ardp.EncodeNodeOpen(ardp.NodeOpen{Recipient: ardp.Open{RecipientNodeID: prepared.receiver.NodeID, RecipientDutyGeneration: prepared.receiver.DutyGeneration, Purpose: uint8(ardp.PurposeDataJoin), Deadline: deadline}, Restriction: ardp.OrdinaryChild})
			if err != nil {
				t.Fatal(err)
			}
			lanes[side], err = parents[side].Open(t.Context(), t.Context(), opened)
			if err != nil {
				t.Fatal("actual ordinary JOIN lane", err)
			}
			secured, err := roletls.OpenRole(t.Context(), lanes[side], prepared.publicKey, deadline)
			if err != nil {
				t.Fatal("actual nested JOIN TLS", err)
			}
			phase("nested role TLS established")
			channels[side] = transport.Retain(secured)
			conn := channels[side]
			if err := conn.SetDeadline(deadline); err != nil {
				t.Fatal(err)
			}
			hello, err := ardp.EncodeHello(ardp.Hello{NetworkID: prepared.facts.NetworkID, StateGeneration: prepared.facts.StateGeneration, StateDigest: prepared.facts.StateDigest, ProfileDigest: prepared.facts.Digest, RecipientNodeID: prepared.receiver.NodeID, RecipientDutyGeneration: prepared.receiver.DutyGeneration, Purpose: ardp.PurposeDataJoin, ChannelNonce: nonce, Deadline: deadline})
			if err != nil {
				t.Fatal(err)
			}
			send(conn, ardp.Frame{Kind: ardp.KindHello, Body: hello})
			send(conn, ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, token...)})
			clear(token)
			ack, err := ardp.ReadFrame(conn)
			if err != nil {
				t.Fatal("actual JOIN admission", side, err)
			}
			status, credit, err := ardp.DecodeAcceptFrame(ack)
			if err != nil || status != 0 || credit != 65536 {
				t.Fatal("actual JOIN acceptance", side, status, credit, err)
			}
			phase("class2 admission accepted")
			requests[side] = ardp.JoinRequest{Nonce: requestNonce, Secret: secret, Context: context, Side: uint8(side + 1), Deadline: deadline}
			body, err := ardp.EncodeJoinRequest(requests[side])
			if err != nil {
				t.Fatal(err)
			}
			send(conn, ardp.Frame{Kind: ardp.KindOperation, Lane: 1, Body: body})
			clear(body)
		})
	}
	setup.Wait()
	if t.Failed() {
		t.FailNow()
	}
	for side, conn := range channels {
		frame := receive(conn, ardp.KindResult)
		status, err := ardp.DecodeJoinResult(frame.Body, requests[side].Nonce)
		if err != nil || status != 0 {
			t.Fatal("actual opposite match result", side, status, err)
		}
	}
	phase("both RESULTs received")
	payloads := [2][]byte{[]byte("independent installed JOIN left to right\x00\xff"), []byte("independent installed JOIN right to left\n\x00")}
	for side, conn := range channels {
		send(conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: payloads[side]})
	}
	for side, conn := range channels {
		frame := receive(conn, ardp.KindBytes)
		if !bytes.Equal(frame.Body, payloads[1-side]) {
			t.Fatal("duplex byte oracle differs", side)
		}
		credit := make([]byte, 4)
		binary.BigEndian.PutUint32(credit, uint32(len(frame.Body)))
		send(conn, ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: credit})
	}
	for side, conn := range channels {
		frame := receive(conn, ardp.KindCredit)
		if binary.BigEndian.Uint32(frame.Body) != uint32(len(payloads[side])) {
			t.Fatal("returned consumed credit differs")
		}
	}
	phase("duplex and credit verified")
	for _, conn := range channels {
		send(conn, ardp.Frame{Kind: ardp.KindEOF, Lane: 1})
	}
	phase("both EOFs emitted")
	for _, conn := range channels {
		receive(conn, ardp.KindEOF)
	}
	phase("both EOFs received")
	send(channels[0], ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}})
	var terminals sync.WaitGroup
	for side, conn := range channels {
		terminals.Go(func() {
			frame := receive(conn, ardp.KindClose)
			t.Logf("terminal side %d: %.3fs", side, time.Since(began).Seconds())
			if !bytes.Equal(frame.Body, []byte{0}) {
				t.Fatal("JOIN terminal differs")
			}
			// The inner authenticated terminal does not prove that the lower
			// receiving handler has joined and emitted its own clean CLOSE. Keep
			// the original parent alive until that exact physical witness arrives.
			// Both sides join independently; neither borrows the other's bound.
			end := min(deadline.UnixNano(), time.Now().Add(time.Second).UnixNano())
			if err := lanes[side].Bound(time.Unix(0, end)); err != nil {
				t.Fatal("original lower cleanup bound", err)
			}
			if err := lanes[side].WaitPeerRetirement(t.Context()); err != nil {
				t.Fatal("original lower peer terminal/join", side, err)
			}
			t.Logf("lower terminal side %d: %.3fs", side, time.Since(began).Seconds())
		})
	}
	terminals.Wait()
	if t.Failed() {
		t.FailNow()
	}
}
