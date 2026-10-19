//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	routereceiver "github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"

	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

func TestRoutePostSpendRefusalRetainsBurnAcrossReopen(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(profile), func(t *testing.T) {
			f, reservations, certificates := newRouteFixture(t, profile)
			holder := routeStock(t, f)
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: routeProcessBudget(t), Domain: 3, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			selected, err := selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: 3, Current: f.current})
			if err != nil {
				t.Fatal(err)
			}
			leg, err := selected.Select()
			if err != nil {
				t.Fatal(err)
			}
			if err := selected.Close(); err != nil {
				t.Fatal(err)
			}
			m := leg.EntryMember
			binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: m.NodeID, DutyGeneration: m.DutyGeneration}
			spendRoot := t.TempDir()
			observe := func() (receiving.Observation, error) { return f.authority.receiver(binding, m.NotAfter()) }
			owner, err := receiving.Open(spendRoot, binding, observe)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = owner.Close() }()
			budget := networkTestBudget(t)
			var releases, spends atomic.Int32
			burnt := make(chan []byte, 1)
			reservations[m.NodeID]()
			server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: leg.Entry, Profile: leg.Profile}, Certificate: certificates[m.NodeID], Admit: func(ctx context.Context, c routereceiver.Channel, raw []byte) (receiving.Grant, error) {
				grant, err := owner.Accept(ctx, admission.ForwardClass, raw, c.Hello.Deadline, func() (func() error, error) {
					release, err := networkTestReservation(t, budget, c.Hello.Deadline)
					if err != nil {
						return nil, err
					}
					return c.HoldReservation(func() error { releases.Add(1); return release() })
				})
				if err == nil {
					spends.Add(1)
					burnt <- bytes.Clone(raw)
					f.clockUnavailable.Store(true)
				}
				return grant, err
			}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = startRoutePrefix(t.Context(), plan, f.authority, holder)
			if err == nil || spends.Load() != 1 {
				t.Fatal("post-spend authority loss accepted or failed before burn", err, spends.Load())
			}
			f.clockUnavailable.Store(false)
			_ = server.Close()
			if releases.Load() != 1 {
				t.Fatal("refused Grant capacity did not return exactly once", releases.Load())
			}
			observation, err := budget.Observe(t.Context())
			if err != nil || observation.ReservedBytes != 0 {
				t.Fatal("refused physical work leaked reserve", err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			owner, err = receiving.Open(spendRoot, binding, observe)
			if err != nil {
				t.Fatal(err)
			}
			var raw []byte
			select {
			case raw = <-burnt:
			default:
				t.Fatal("actual presented token not captured")
			}
			defer clear(raw)
			deadline := time.Now().Add(5 * time.Second)
			grant, err := owner.Accept(t.Context(), admission.ForwardClass, raw, deadline, func() (func() error, error) { return networkTestReservation(t, budget, deadline) })
			if err == nil {
				_ = grant.Release()
				t.Fatal("reopen restored post-spend refused token")
			}
			observation, err = budget.Observe(t.Context())
			if err != nil || observation.ReservedBytes != 0 {
				t.Fatal("replay refusal leaked rollback reserve", err)
			}
		})
	}
}

func TestRouteInvalidHELLORefusesBeforeAdmission(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(profile), func(t *testing.T) {
			f, reservations, certificates := newRouteFixture(t, profile)
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			m, err := view.Member([32]byte{12}, view.ObservedAt())
			if err != nil {
				t.Fatal(err)
			}
			duty, err := view.RetainDuty(m.NodeID, view.ObservedAt())
			if err != nil {
				t.Fatal(err)
			}
			binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: m.NodeID, DutyGeneration: m.DutyGeneration}
			owner, err := receiving.Open(t.TempDir(), binding, func() (receiving.Observation, error) { return f.authority.receiver(binding, m.NotAfter()) })
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			budget := networkTestBudget(t)
			var admissionCalls atomic.Int32
			reservations[m.NodeID]()
			server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[m.NodeID], Admit: func(ctx context.Context, c routereceiver.Channel, raw []byte) (receiving.Grant, error) {
				admissionCalls.Add(1)
				return owner.Accept(ctx, admission.ForwardClass, raw, c.Hello.Deadline, func() (func() error, error) {
					release, err := networkTestReservation(t, budget, c.Hello.Deadline)
					if err != nil {
						return nil, err
					}
					return c.HoldReservation(release)
				})
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := server.Close(); err != nil {
					t.Error(err)
				}
			}()
			for _, fault := range []string{"recipient", "duty", "purpose", "state", "profile", "deadline"} {
				t.Run(fault, func(t *testing.T) {
					conn, err := routeTestOpenEndpoint(t.Context(), transport.ClosedRoleCarrierRequest{CarrierProfile: profile, Endpoint: m.Endpoint, ExpectedServer: m.PublicKey, Deadline: time.Now().Add(3 * time.Second)})
					if err != nil {
						t.Fatal(err)
					}
					defer conn.Close()
					h := ardp.Hello{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, RecipientNodeID: m.NodeID, RecipientDutyGeneration: m.DutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{1}, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}
					switch fault {
					case "recipient":
						h.RecipientNodeID = [32]byte{99}
					case "duty":
						h.RecipientDutyGeneration++
					case "purpose":
						h.Purpose = ardp.PurposeDataJoin
					case "state":
						h.StateGeneration[0] ^= 1
					case "profile":
						h.ProfileDigest[0] ^= 1
					case "deadline":
						h.Deadline = m.NotAfter().Add(time.Second)
					}
					body, err := ardp.EncodeHello(h)
					if err != nil {
						t.Fatal(err)
					}
					if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
						t.Fatal(err)
					}
					if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
						t.Fatal(err)
					}
					if _, err := ardp.ReadFrame(conn); err == nil || errors.Is(err, os.ErrDeadlineExceeded) {
						t.Fatal("invalid HELLO reached acceptance or waited for ADMIT", err)
					}
					if admissionCalls.Load() != 0 {
						t.Fatal("invalid HELLO reached receiving Admission")
					}
				})
			}
			observation, err := budget.Observe(t.Context())
			if err != nil || observation.ReservedBytes != 0 {
				t.Fatal("refusal reserved physical capacity", observation, err)
			}
		})
	}
}

// Signed historical QUIC records remain genuine Network facts. They cannot
// authorize current Route effects, including a TCP reinterpretation or Stock
// presentation. The companion positive TCP scenarios use these same owners.
func TestRouteSignedQUICRefusesBeforeTransportAndPresentation(t *testing.T) {
	tcp, udpReservation := routeRefusalSockets(t)
	f, reservations, certificates := newRoleRouteFixtureConfigured(t, transport.ClosedCarrierQUIC, 3, false, false,
		func(f *networkAdmissionFixture, reservations map[[32]byte]func(), _ map[[32]byte]tls.Certificate) {
			id := [32]byte{12}
			reservations[id]()
			for index := range f.spec.Nodes {
				if f.spec.Nodes[index].NodeID == id {
					f.spec.Nodes[index].Endpoint = tcp.Addr().String()
				}
			}
			reservations[id] = func() { _ = udpReservation.Close() }
		})
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	m, err := view.Member([32]byte{12}, view.ObservedAt())
	if err != nil || m.CarrierProfile != string(transport.ClosedCarrierQUIC) {
		t.Fatal("signed historical profile was lost or rewritten", err)
	}
	duty, err := view.RetainDuty(m.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	// Keep TCP occupied and independently release the fixture UDP reservation.
	// An unsupported-profile error must precede either socket acquisition.
	reservations[m.NodeID]()
	admitted := false
	server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{
		Authority:   role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding},
		Certificate: certificates[m.NodeID],
		Admit: func(context.Context, routereceiver.Channel, []byte) (receiving.Grant, error) {
			admitted = true
			return receiving.Grant{}, nil
		},
	})
	if server != nil || err == nil || !strings.Contains(err.Error(), "Carrier profile is unsupported") || admitted {
		t.Fatal("deferred Carrier reached listener or Admission", err, admitted)
	}
	udp, err := net.ListenPacket("udp", m.Endpoint)
	if err != nil {
		t.Fatal("refusal acquired or retained the UDP endpoint", err)
	}
	defer udp.Close()
	holder := routeStock(t, f)
	before := holder.Status()
	root := t.TempDir()
	plan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: routeProcessBudget(t), Domain: 3,
		Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	handle, err := startRoutePrefix(t.Context(), plan, f.authority, holder)
	if err == nil {
		_ = handle.close()
		t.Fatal("deferred-only signed Entry set opened a prefix")
	}
	if after := holder.Status(); after != before || !after.Accepted {
		t.Fatal("unsupported selection changed genuine Stock", before, after)
	}
	// Permission quota alone cannot establish remaining token stock. There is
	// exactly one genuine Forward token per candidate; consume each only after
	// the refused operation as an independent oracle that none was presented.
	// These local test presentations grant no Route or transport permission.
	for index := byte(12); index < 16; index++ {
		raw, err := holder.Take(t.Context(), stock.Presentation{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration,
			StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, RecipientNodeID: [32]byte{index}, RecipientDutyGeneration: 9,
			ChannelNonce: [32]byte{index}, Deadline: plan.Deadline}, uint8(admission.ForwardClass))
		present := len(raw) != 0
		clear(raw)
		if err != nil || !present {
			t.Fatal("refused Route consumed an original candidate token", index, err)
		}
	}
	if err := tcp.(*net.TCPListener).SetDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	conn, err := tcp.Accept()
	if conn != nil {
		_ = conn.Close()
		t.Fatal("refusal attempted TCP fallback")
	}
	if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatal("TCP sentinel lost its original socket", err)
	}
	if err := udp.SetReadDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := udp.ReadFrom(make([]byte, 1)); err == nil {
		t.Fatal("refused prefix emitted UDP")
	} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatal("UDP sentinel lost its original socket", err)
	}
	after, err := f.current()
	if err != nil || after.MatchDuty(duty, after.ObservedAt()) != nil {
		t.Fatal("Route refusal mutated original Network authority", err)
	}
}

// The kernel allocates TCP and UDP ports independently. Select both original
// sentinels before constructing or signing Network facts; a UDP-only port is
// not evidence that its matching TCP endpoint is available.
func routeRefusalSockets(t *testing.T) (net.Listener, net.PacketConn) {
	t.Helper()
	for attempt := 0; attempt < 32; attempt++ {
		tcp, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		udp, err := net.ListenPacket("udp", tcp.Addr().String())
		if err == nil {
			t.Cleanup(func() { _ = tcp.Close(); _ = udp.Close() })
			return tcp, udp
		}
		_ = tcp.Close()
		if !errors.Is(err, syscall.EADDRINUSE) {
			t.Fatal(err)
		}
	}
	t.Fatal("invalid environment: no jointly available TCP/UDP sentinel port")
	return nil, nil
}
