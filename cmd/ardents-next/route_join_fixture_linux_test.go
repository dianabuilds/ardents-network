//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	routereceiver "github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"

	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// newJoinRouteFixture supplies signed records and profiles through real new
// State acceptance. The fixture owns no successful authority substitute.
func newJoinRouteFixture(t *testing.T, profile transport.CarrierProfile) (*networkAdmissionFixture, map[[32]byte]func(), map[[32]byte]tls.Certificate) {
	t.Helper()
	if profile != transport.ClosedCarrierTCP && profile != transport.ClosedCarrierQUIC {
		t.Fatal("unselected JOIN Carrier")
	}
	reservations := make(map[[32]byte]func())
	certificates := make(map[[32]byte]tls.Certificate)
	f := newNetworkAdmissionFixture(t, func(f *networkAdmissionFixture) {
		// Keep the independently signed issuer (11), replace the initial receiver
		// with Source Entry (12), and add the remaining opposite-role duties.
		f.spec.Nodes = f.spec.Nodes[:1]
		for idByte := byte(12); idByte <= 21; idByte++ {
			id := [32]byte{idByte}
			_, key, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { clear(key) })
			var address string
			if profile == transport.ClosedCarrierTCP {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				address = listener.Addr().String()
				reservations[id] = func() { _ = listener.Close() }
			} else {
				socket, err := net.ListenPacket("udp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				address = socket.LocalAddr().String()
				reservations[id] = func() { _ = socket.Close() }
			}
			t.Cleanup(reservations[id])
			domain, subrole := byte(1), byte(1)
			switch {
			case idByte < 16:
				if idByte >= 14 {
					subrole = 2
				}
			case idByte < 20:
				domain = 3
				if idByte >= 18 {
					subrole = 2
				}
			default:
				domain = 2
				subrole = 4
			}
			f.spec.Nodes = append(f.spec.Nodes, networkfixture.ClosedNode{RecordSpec: networkfixture.RecordSpec{NodeID: id, Generation: 9, ValidFrom: f.spec.NotBefore, ValidUntil: f.spec.NotAfter, Endpoint: address, Carrier: string(profile), Capability: 2, Capacity: 1, PrivateKey: key}, RoleDomain: domain, Subrole: subrole})
			certificates[id] = routeTestCertificate(t, key)
		}
	})
	return f, reservations, certificates
}

// joinRouteStock prepares one independently allocated principal's genuine
// blinded class-2 batch, with one token for each of the ten actual receivers.
// No token is presented until the real transport consumer authenticates HELLO.
func joinRouteStock(t *testing.T, f *networkAdmissionFixture, role admission.AllocationRole) *stock.Owner {
	t.Helper()
	if role != admission.AllocationUser && role != admission.AllocationPublisher {
		t.Fatal("JOIN principal role unavailable")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	holder, err := stock.Open(root, role, f.authority.observe)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := holder.Close(); err != nil {
			t.Error(err)
		}
	})
	maxima := [3]uint32{0, 10, 0}
	raw, digest, err := holder.Request(maxima)
	if err != nil {
		t.Fatal(err)
	}
	request, err := allocation.Prepare(raw, f.profile.NetworkID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	decision, err := request.Decide(nil, f.profile.IssuanceAuthorityKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(t.TempDir(), "allocation")
	if err := os.WriteFile(journal, decision.Journal(), 0600); err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(committed, decision.Journal()) {
		t.Fatal("JOIN allocation durable readback", err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(f.spec.Authority, admission.PermissionTranscript(permission)))
	encoded, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Import(digest, encoded); err != nil {
		t.Fatal(err)
	}
	if status := holder.Status(); !status.Accepted || status.Remaining != maxima {
		t.Fatalf("JOIN allocation maxima changed: %+v", status)
	}
	var challenges []token.ClosedTokenContext
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	for idByte := byte(12); idByte <= 21; idByte++ {
		id := [32]byte{idByte}
		member, err := view.Member(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		challenges = append(challenges, token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: id, ReceiverDutyGeneration: member.DutyGeneration, Class: 2, WindowStart: time.Now().UTC().Truncate(time.Hour)})
	}
	attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{31, byte(role)}, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: f.profile.NotAfter})
	if err != nil {
		t.Fatal(err)
	}
	batch, _, err := attempt.Request()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := admission.DecodeClosedTokenBatch(batch)
	if err != nil || decoded.Class != 2 || len(decoded.BlindedRequests) != 10 {
		t.Fatal("JOIN blinded batch class/count", err)
	}
	issued := issuer.IssueCurrent(t.Context(), f.plan, batch, quota.Bootstrap, func() (admission.AuthorityFacts, time.Time, error) {
		return f.authority.issuer(f.plan.KeyBinding.Signer)
	})
	if issued.Outcome != "issued-offline" {
		t.Fatal(issued)
	}
	response, err := admission.DecodeClosedTokenBatchResult(issued.Response)
	if err != nil || response.Status != admission.ClosedTokenIssued || len(response.Signatures) != 10 {
		t.Fatal("JOIN blind issuance result class/count", err)
	}
	if err := attempt.Complete(issued.Response, nil); err != nil {
		t.Fatal(err)
	}
	return holder
}

// startJoinRouteReceivers owns distinct receiving histories and Hosting budgets.
// Each accepted Channel retains its real reservation until physical join.
func startJoinRouteReceivers(t *testing.T, f *networkAdmissionFixture, reservations map[[32]byte]func(), certificates map[[32]byte]tls.Certificate) ([]*routereceiver.Receiver, []*hosting.Budget) {
	t.Helper()
	var receivers []*routereceiver.Receiver
	var budgets []*hosting.Budget
	for idByte := byte(12); idByte <= 21; idByte++ {
		id := [32]byte{idByte}
		view, err := f.current()
		if err != nil {
			t.Fatal(err)
		}
		member, err := view.Member(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		duty, err := view.RetainDuty(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: id, DutyGeneration: member.DutyGeneration}
		owner, err := receiving.Open(t.TempDir(), binding, func() (receiving.Observation, error) { return f.authority.receiver(binding, member.NotAfter()) })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := owner.Close(); err != nil {
				t.Error(err)
			}
		})
		budget := networkTestBudget(t)
		budgets = append(budgets, budget)
		reservation, ok := reservations[id]
		if !ok {
			t.Fatal("JOIN endpoint reservation absent")
		}
		reservation()
		certificate, ok := certificates[id]
		if !ok {
			t.Fatal("JOIN exact certificate absent")
		}
		receiver, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificate, Admit: func(ctx context.Context, channel routereceiver.Channel, raw []byte) (receiving.Grant, error) {
			return owner.Accept(ctx, admission.ForwardClass, raw, channel.Hello.Deadline, func() (func() error, error) {
				release, err := networkTestReservation(t, budget, channel.Hello.Deadline)
				if err != nil {
					return nil, err
				}
				return channel.HoldReservation(release)
			})
		}})
		if err != nil {
			t.Fatal(err)
		}
		receivers = append(receivers, receiver)
		t.Cleanup(func() {
			if err := receiver.Close(); err != nil {
				t.Error("JOIN receiver physical close", err)
			}
		})
	}
	return receivers, budgets
}

func TestJoinRouteFixtureCurrentDutiesAndStock(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, reservations, certificates := newJoinRouteFixture(t, profile)
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			if len(view.Members()) != 11 || len(reservations) != 10 || len(certificates) != 10 {
				t.Fatal("JOIN fixture duty/endpoint count changed")
			}
			var counts [4][7]int
			for idByte := byte(12); idByte <= 21; idByte++ {
				id := [32]byte{idByte}
				member, err := view.Member(id, view.ObservedAt())
				if err != nil {
					t.Fatal(err)
				}
				if member.DutyGeneration != 9 || member.CarrierProfile != string(profile) {
					t.Fatal("JOIN duty binding differs")
				}
				key, ok := certificates[id].PrivateKey.(ed25519.PrivateKey)
				if !ok || !bytes.Equal(key.Public().(ed25519.PublicKey), member.PublicKey[:]) {
					t.Fatal("JOIN certificate differs from signed exact member key")
				}
				counts[member.RoleDomain-1][member.Subrole]++
			}
			if counts[0][1] != 2 || counts[0][2] != 2 || counts[2][1] != 2 || counts[2][2] != 2 || counts[1][4] != 2 {
				t.Fatal("JOIN opposite-role/terminal duties changed", counts)
			}
			for _, role := range []admission.AllocationRole{admission.AllocationUser, admission.AllocationPublisher} {
				holder := joinRouteStock(t, f, role)
				status := holder.Status()
				if !status.Accepted || status.Pending || status.Busy || status.Closed || status.Remaining != [3]uint32{} || status.BootstrapRemaining != 1 {
					t.Fatalf("JOIN stock did not complete exactly one ten-token batch: %+v", status)
				}
				// This is quota/stock preparation evidence only. No TLS HELLO, presentation,
				// receiving spend, JOIN pairing or forwarding is claimed by this smoke.
			}
		})
	}
}
