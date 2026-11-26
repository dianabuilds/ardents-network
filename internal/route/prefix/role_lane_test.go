package prefix

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/route/channel"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	roletls "github.com/dianabuilds/ardents-network/internal/route/transport/tls"
)

// Every role terminal opens its lower lane through one mechanism. The physical
// Interior parent receives the exact recipient duty, purpose and bound, and an
// ended setup closes only the lane whose interruption it armed. These controls
// supply no successful Network observation, Stock, ACK or authority.
func TestRoleLaneCarriesExactRecipientAndInterruptionClosesOnlyItsLane(t *testing.T) {
	for name, purpose := range map[string]ardp.Purpose{"issuer": ardp.PurposeIssuer, "reachability": ardp.PurposeReachability,
		"introduction": ardp.PurposeIntroduction, "submission": ardp.PurposeSubmission, "join": ardp.PurposeDataJoin} {
		t.Run(name, func(t *testing.T) {
			p, _, interiorPeer, _, cleanup := terminalPhysicalPrefix(t)
			defer cleanup()
			frames := make(chan ardp.Frame, 4)
			readerDone := make(chan struct{})
			go func() {
				defer close(readerDone)
				for {
					frame, err := ardp.ReadFrame(interiorPeer)
					if err != nil {
						return
					}
					frames <- frame
				}
			}()
			defer func() { _ = interiorPeer.Close(); <-readerDone }()
			var node [32]byte
			node[0], node[31] = 7, 9
			duty := network.RetainedDuty{NodeID: node, RecordGeneration: 41}
			end := time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)
			intact, cancelIntact := context.WithCancel(t.Context())
			defer cancelIntact()
			kept, keptInterruption, err := p.openRoleLane(intact, t.Context(), duty, purpose, end)
			if err != nil {
				t.Fatal(err)
			}
			first := openedRecipient(t, frames, duty, purpose, end)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			lane, interruption, err := p.openRoleLane(ctx, t.Context(), duty, purpose, end)
			if err != nil {
				t.Fatal(err)
			}
			if second := openedRecipient(t, frames, duty, purpose, end); second == first {
				t.Fatal("two role lanes shared one physical lane identifier")
			}
			cancel()
			if !interruption.join() {
				t.Fatal("ended setup did not run the interruption of its own lane")
			}
			if _, joined := lane.CleanupResult(); !joined {
				t.Fatal("interrupted role lane did not join its physical cleanup")
			}
			if keptInterruption.join() {
				t.Fatal("another lane's interruption ran")
			}
			if _, joined := kept.CleanupResult(); joined {
				t.Fatal("one lane's interruption closed another role lane")
			}
			_ = kept.Close()
			kept.Finish()
		})
	}
}

func openedRecipient(t *testing.T, frames <-chan ardp.Frame, duty network.RetainedDuty, purpose ardp.Purpose, end time.Time) uint32 {
	t.Helper()
	frame := <-frames
	opened, err := ardp.DecodeOpen(frame.Body)
	if err != nil || frame.Kind != ardp.KindOpen || frame.Lane == 0 || opened.RecipientNodeID != duty.NodeID ||
		opened.RecipientDutyGeneration != duty.RecordGeneration || opened.Purpose != uint8(purpose) || !opened.Deadline.Equal(end) {
		t.Fatalf("role lane OPEN changed its exact recipient, purpose or bound: %+v %v", frame, err)
	}
	return frame.Lane
}

// A presented role lane hands its retained connection to the owner on every
// path after TLS succeeded, so a later refusal still closes it there. A refused
// handshake returns no connection. The peer supplies TLS mechanics only; no
// member observation, HELLO acceptance or authority succeeds here.
func TestPresentedRoleLaneLeavesItsRetainedConnectionToTheOwner(t *testing.T) {
	for _, fault := range []string{"handshake", "observation"} {
		t.Run(fault, func(t *testing.T) {
			lane, key, peer := roleLanePeer(t, fault == "handshake")
			p := &Prefix{config: Config{Present: func(context.Context, ardp.Hello) ([]byte, error) {
				t.Error("refused role lane reached presentation")
				return nil, errors.New("unexpected presentation")
			}}}
			cause := errors.New("original Network observation refused")
			authority := role.Authority{Current: func() (network.RuntimeView, error) { return network.RuntimeView{}, cause }}
			end := time.Now().Add(5 * time.Second)
			conn, err := p.presentRoleLane(t.Context(), t.Context(), lane, authority, network.Member{PublicKey: key}, ardp.PurposeIssuer, end, end, false)
			server := <-peer
			if server.err != nil {
				t.Fatal(server.err)
			}
			t.Cleanup(func() {
				_ = server.raw.Close()
				if server.session != nil {
					_ = server.session.Close()
				}
			})
			if fault == "handshake" {
				if conn != nil || err == nil {
					t.Fatal("refused role handshake returned a connection", err)
				}
				return
			}
			if conn == nil || !errors.Is(err, cause) {
				t.Fatal("refused role observation lost its cause or its retained connection", err)
			}
			// Nothing was presented, and the refused connection stays with its
			// owner: the peer still waits instead of observing a close.
			if err := server.secured.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
				t.Fatal(err)
			}
			if n, err := server.secured.Read(make([]byte, 1)); n != 0 || !timedOut(err) {
				t.Fatal("refused role lane presented output or closed the retained connection", n, err)
			}
			if err := server.secured.SetReadDeadline(time.Time{}); err != nil {
				t.Fatal(err)
			}
			closed := make(chan error, 1)
			go func() {
				n, err := server.secured.Read(make([]byte, 1))
				if n != 0 {
					err = errors.New("unexpected role lane output")
				}
				closed <- err
			}()
			if err := conn.Close(); err != nil {
				t.Fatal(err)
			}
			if err := <-closed; err == nil || timedOut(err) {
				t.Fatal("owner Close did not terminate the retained role connection", err)
			}
			if err := conn.Close(); err != nil {
				t.Fatal("repeated owner Close changed the retained result", err)
			}
		})
	}
}

func timedOut(err error) bool {
	var networkErr net.Error
	return errors.As(err, &networkErr) && networkErr.Timeout()
}

type acceptedRolePeer struct {
	secured *tls.Conn
	session *framing.Session
	raw     net.Conn
	err     error
}

// roleLanePeer connects one joined lane to an actual role TLS peer. Both ends
// carry the same dedicated framing, as a relay does for a role lane. A
// refusing peer closes before TLS. It supplies transport mechanics only.
func roleLanePeer(t *testing.T, refuse bool) (*framing.Lane, [32]byte, <-chan acceptedRolePeer) {
	t.Helper()
	certificate, key := roleLaneCertificate(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	end := time.Now().Add(5 * time.Second)
	peer := make(chan acceptedRolePeer, 1)
	go func() {
		raw, err := listener.Accept()
		if err != nil {
			peer <- acceptedRolePeer{err: err}
			return
		}
		if refuse {
			peer <- acceptedRolePeer{raw: raw, err: raw.Close()}
			return
		}
		session, incoming, err := framing.PrepareJoined(context.Background(), raw, end, 1<<20, nil, framing.NewBudget(4<<20))
		if err != nil {
			_ = raw.Close()
			peer <- acceptedRolePeer{err: err}
			return
		}
		session.Start()
		secured, err := roletls.AcceptRole(context.Background(), incoming, certificate, end)
		if err != nil {
			_ = raw.Close()
			_ = session.Close()
			peer <- acceptedRolePeer{err: err}
			return
		}
		peer <- acceptedRolePeer{secured: secured, session: session, raw: raw}
	}()
	raw, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	owner, lane, err := framing.PrepareJoined(t.Context(), raw, end, 1<<20, nil, framing.NewBudget(4<<20))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	t.Cleanup(func() { _ = raw.Close() })
	owner.Start()
	return lane, key, peer
}
