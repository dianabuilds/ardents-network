package outer

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"math/big"
	"net"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

// This peer adapter carries real inner TLS over the bridge's ARDP frames.
// Credential verification is isolated; admission, exporter binding and lane
// authority are real. It does not qualify credentials or either Carrier.
type queuedLanePeer struct {
	net.Conn
	bridge  *route.ClosedOuterBridge
	id      uint32
	pending []byte
}

func (peer *queuedLanePeer) Write(value []byte) (int, error) {
	_, err := peer.bridge.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: peer.id, Body: append([]byte(nil), value...)})
	if err != nil {
		return 0, err
	}
	return len(value), nil
}

func (peer *queuedLanePeer) Read(value []byte) (int, error) {
	for len(peer.pending) == 0 {
		frame, err := ardp.ReadFrame(peer.Conn)
		if err != nil {
			return 0, err
		}
		if frame.Kind != ardp.KindBytes || frame.Lane != peer.id {
			return 0, errors.New("unexpected TLS outer frame")
		}
		peer.pending = frame.Body
	}
	n := copy(value, peer.pending)
	peer.pending = peer.pending[n:]
	return n, nil
}

func queuedAdmittedLanes(t *testing.T) (*writer, net.Conn, *route.ClosedOuterBridge, []*route.ClosedOuterBridgeLane) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	receiver := route.ClosedOuterReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 7, RoleDomain: 1, Subrole: 2, Deadline: now.Add(time.Hour)}
	limits, err := route.NewClosedDutyLimits(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handshake, err := route.NewClosedOuterHandshake(receiver, limits, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	local, remote := net.Pipe()
	t.Cleanup(func() { local.Close(); remote.Close(); handshake.Close() })
	owner := &writer{connection: local}
	bridge, err := route.NewClosedOuterBridge(handshake, owner.update, owner.write)
	if err != nil {
		t.Fatal(err)
	}
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{8}, Deadline: receiver.Deadline}
	body, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan error, 1)
	go func() { _, err := ardp.ReadFrame(remote); accepted <- err }()
	if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	if err := <-accepted; err != nil {
		t.Fatal(err)
	}
	spends, err := replay.Open(t.TempDir(), replay.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := spends.Close(); err != nil {
			t.Error(err)
		}
	})
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now, NotAfter: receiver.Deadline}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	var lanes []*route.ClosedOuterBridgeLane
	for _, id := range []uint32{1, 3, 5} {
		end := now.Add(30 * time.Minute)
		body, err := route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, Deadline: end}, route.ClosedChildOrdinary)
		if err != nil {
			t.Fatal(err)
		}
		lane, err := bridge.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: body})
		if err != nil {
			t.Fatal(err)
		}
		secured := tls.Server(lane, &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS13, NextProtos: []string{carrier.ClosedRouteProfile}})
		client := tls.Client(&queuedLanePeer{Conn: remote, bridge: bridge, id: id}, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13, NextProtos: []string{carrier.ClosedRouteProfile}})
		complete := make(chan error, 1)
		go func() { complete <- client.HandshakeContext(t.Context()) }()
		if err := secured.HandshakeContext(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := <-complete; err != nil {
			t.Fatal(err)
		}
		if err := lane.BeginInnerHello(); err != nil {
			t.Fatal(err)
		}
		inner := hello
		inner.ChannelNonce[0] = byte(10 + id)
		inner.Deadline = end
		if err := lane.Activate(inner); err != nil {
			t.Fatal(err)
		}
		exporter, err := carrier.ClosedRoleTLSExporter(secured)
		if err != nil {
			t.Fatal(err)
		}
		role := route.ClosedRoleReceiver{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, NodeID: receiver.NodeID, RecordDigest: receiver.RecordDigest, DutyGeneration: receiver.DutyGeneration, RoleDomain: receiver.RoleDomain, Subrole: receiver.Subrole, ExpectedPurpose: ardp.PurposeForwarding, NotAfter: receiver.Deadline}
		admission, err := route.NewClosedAdmissionChannel(role, spends, limits, exporter, func(route.ClosedAdmissionVerification) (route.ClosedAdmissionApproval, error) {
			return route.ClosedAdmissionApproval{Window: now.Truncate(time.Hour)}, nil
		}, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		body, err = ardp.EncodeHello(inner)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := admission.Accept(ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
			t.Fatal(err)
		}
		lease, err := admission.Accept(ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, bytes.Repeat([]byte{byte(id)}, 354)...)})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := lease.Release(); err != nil {
				t.Error(err)
			}
		})
		if err := lane.Admit(&lease, secured); err != nil {
			t.Fatal(err)
		}
		if err := lane.SetDeadline(time.Time{}); err != nil {
			t.Fatal(err)
		}
		lanes = append(lanes, lane)
	}
	if err := remote.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return owner, remote, bridge, lanes
}

func waitQueuedLane(t *testing.T, owner *writer, id uint32) {
	t.Helper()
	end := time.Now().Add(time.Second)
	for time.Now().Before(end) {
		owner.state.Lock()
		found := false
		for _, queue := range [][]*writeRequest{owner.data, owner.controls, owner.terminals} {
			for _, request := range queue {
				found = found || request.frame.Lane == id
			}
		}
		owner.state.Unlock()
		if found {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("lane never entered writer queue")
}

func TestClosedOuterQueuedLaneDeadlineReturnsBeforeSibling(t *testing.T) {
	for _, change := range []string{"expires", "shortened", "extended", "credit", "terminal", "peer-close"} {
		t.Run(change, func(t *testing.T) {
			owner, peer, bridge, lanes := queuedAdmittedLanes(t)
			var helpers sync.WaitGroup
			t.Cleanup(func() { owner.connection.Close(); peer.Close(); helpers.Wait() })
			a, b := make(chan error, 1), make(chan error, 1)
			helpers.Go(func() { _, err := lanes[0].Write([]byte{41}); a <- err })
			// Receiving just the header leaves A's physical body write active.
			if _, err := io.ReadFull(peer, make([]byte, ardp.HeaderSize)); err != nil {
				t.Fatal(err)
			}
			end := time.Now().Add(200 * time.Millisecond)
			if change == "shortened" || change == "peer-close" {
				end = time.Now().Add(time.Minute)
			}
			if err := lanes[1].SetWriteDeadline(end); err != nil {
				t.Fatal(err)
			}
			if change == "credit" {
				if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 3, Body: []byte{42}}); err != nil {
					t.Fatal(err)
				}
				helpers.Go(func() { _, err := lanes[1].Read(make([]byte, 1)); b <- err })
			} else if change == "terminal" {
				helpers.Go(func() { b <- lanes[1].Close() })
			} else {
				helpers.Go(func() { _, err := lanes[1].Write([]byte{42}); b <- err })
			}
			waitQueuedLane(t, owner, 3)
			if change == "extended" {
				if err := lanes[1].SetWriteDeadline(time.Time{}); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-b:
					t.Fatalf("obsolete deadline completed extended writer: %v", err)
				case <-time.After(250 * time.Millisecond):
				}
			}
			if change == "shortened" || change == "extended" {
				if err := lanes[1].SetWriteDeadline(time.Now()); err != nil {
					t.Fatal(err)
				}
			}
			if change == "peer-close" {
				if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindClose, Lane: 3, Body: []byte{0}}); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-b:
				if !errors.Is(err, os.ErrDeadlineExceeded) {
					t.Fatalf("queued deadline: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("queued Write did not return while sibling remained active")
			}
			select {
			case err := <-a:
				t.Fatalf("queued timeout interrupted sibling: %v", err)
			default:
			}
			var body [1]byte
			if _, err := peer.Read(body[:]); err != nil || body[0] != 41 {
				t.Fatalf("sibling body: %v %v", body, err)
			}
			if err := <-a; err != nil {
				t.Fatal(err)
			}
			received := make(chan ardp.Frame, 1)
			helpers.Go(func() { frame, _ := ardp.ReadFrame(peer); received <- frame })
			if _, err := lanes[2].Write([]byte{43}); err != nil {
				t.Fatal(err)
			}
			if frame := <-received; frame.Lane != 5 || !bytes.Equal(frame.Body, []byte{43}) {
				t.Fatalf("canceled work emitted or healthy sibling changed: %+v", frame)
			}
			// The owner must drain and release every request, including canceled
			// controls/terminals. Cleanup above joins all caller goroutines.
			until := time.Now().Add(time.Second)
			for {
				owner.state.Lock()
				idle := !owner.running && owner.active == nil && len(owner.data)+len(owner.controls)+len(owner.terminals) == 0
				owner.state.Unlock()
				if idle {
					break
				}
				if time.Now().After(until) {
					t.Fatal("writer retained work after completion")
				}
				runtime.Gosched()
			}
		})
	}
}

// Selection, physical close and expiry may race. A returned request may no
// longer reference caller memory, whether it was removed or selected first.
func TestClosedOuterQueuedCancellationRacesSelectionAndClose(t *testing.T) {
	for attempt := 0; attempt < 50; attempt++ {
		local, peer := net.Pipe()
		owner := &writer{connection: local}
		owner.writer.Lock()
		var unlock sync.Once
		release := func() { unlock.Do(owner.writer.Unlock) }
		var mu sync.Mutex
		end := time.Now().Add(time.Minute)
		body := []byte{1}
		done := make(chan error, 1)
		var helpers sync.WaitGroup
		t.Cleanup(func() { release(); local.Close(); peer.Close(); helpers.Wait() })
		helpers.Go(func() {
			done <- owner.write(ardp.Frame{Kind: ardp.KindBytes, Lane: 3, Body: body}, func() time.Time { mu.Lock(); defer mu.Unlock(); return end }, false, false)
		})
		waitQueuedLane(t, owner, 3)
		start := make(chan struct{})
		helpers.Go(func() {
			<-start
			mu.Lock()
			end = time.Now()
			mu.Unlock()
			_ = owner.update(3, time.Now())
		})
		helpers.Go(func() { <-start; release() })
		helpers.Go(func() { <-start; _ = local.Close() })
		close(start)
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("canceled/closed request succeeded")
			}
		case <-time.After(time.Second):
			t.Fatal("cancellation race retained caller")
		}
		body[0] = 99
		local.Close()
		peer.Close()
		helpers.Wait()
		until := time.Now().Add(time.Second)
		for {
			owner.state.Lock()
			idle := !owner.running && owner.active == nil && len(owner.data)+len(owner.controls)+len(owner.terminals) == 0
			owner.state.Unlock()
			if idle {
				break
			}
			if time.Now().After(until) {
				t.Fatal("race retained drain")
			}
			runtime.Gosched()
		}
	}
}
