//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The driver owns only signed inputs and a real client. Both initial admission
// and parent refill execute inside the genuine original Node command, with its
// native process guard, producer, receiving journal and Hosting reservations.
func TestNativeNodeParentRefillAndReopenBothCarriers(t *testing.T) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, mode := range []string{"refill", "capacity-before-spend"} {
				t.Run(mode, func(t *testing.T) {
					f, sockets, certificates := newRouteFixture(t, carrier)
					bundle, err := networkfixture.BuildClosed(f.spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := joinProcessClock(t)
					defer closeClock()
					certificate := certificates[[32]byte{12}]
					certPath, keyPath := filepath.Join(t.TempDir(), "certificate"), filepath.Join(t.TempDir(), "key")
					if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
						t.Fatal(err)
					}
					key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
					if err != nil {
						t.Fatal(err)
					}
					encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
					if err := os.WriteFile(keyPath, encoded, 0600); err != nil {
						t.Fatal(err)
					}
					clear(key)
					clear(encoded)
					root, spend, budgetRoot := filepath.Join(t.TempDir(), "participation"), t.TempDir(), routeProcessBudget(t)
					work, termination := hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}
					plan := map[string]any{"network": seedNativeNodeState(t, f, bundle, clock), "node_id": [32]byte{12}, "process_profile": processresource.Profile, "participation_root": root, "spend_root": spend, "hosting_root": budgetRoot, "certificate": certPath, "private_key": keyPath, "work": work, "termination": termination, "retirement_wait_ms": 5000}
					sockets[[32]byte{12}]()
					process := startNativeForwarding(t, binary, hostingConfig(t, plan), 90)
					holder := routeRoleStock(t, f, false, true)
					view, err := f.current()
					if err != nil {
						t.Fatal(err)
					}
					member, err := view.Member([32]byte{12}, view.ObservedAt())
					if err != nil {
						t.Fatal(err)
					}
					end := time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)
					conn, err := routeTestOpenEndpoint(t.Context(), transport.ClosedRoleCarrierRequest{CarrierProfile: carrier, Endpoint: member.Endpoint, ExpectedServer: member.PublicKey, Deadline: end})
					if err != nil {
						t.Fatal(err)
					}
					defer conn.Close()
					if err := conn.SetDeadline(end); err != nil {
						t.Fatal(err)
					}
					hello := ardp.Hello{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration, Purpose: ardp.PurposeForwarding, Deadline: end}
					if _, err := rand.Read(hello.ChannelNonce[:]); err != nil {
						t.Fatal(err)
					}
					body, err := ardp.EncodeHello(hello)
					if err != nil {
						t.Fatal(err)
					}
					if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
						t.Fatal(err)
					}
					presentation := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
					take := func() []byte {
						if err := f.authority.presentation(presentation); err != nil {
							t.Fatal(err)
						}
						raw, err := holder.Take(t.Context(), presentation, 2)
						if err != nil {
							t.Fatal(err)
						}
						if err := f.authority.presentation(presentation); err != nil {
							t.Fatal(err)
						}
						return raw
					}
					first, second := take(), take()
					defer clear(first)
					defer clear(second)
					if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, first...)}); err != nil {
						t.Fatal(err)
					}
					ack, err := ardp.ReadFrame(conn)
					if err != nil {
						t.Fatal(err)
					}
					status, credit, err := ardp.DecodeAcceptFrame(ack)
					if err != nil || status != 0 || credit != 65536 {
						t.Fatal("original Node admission refused", status, credit, err)
					}
					firstSpend, err := os.ReadFile(filepath.Join(spend, "closed-token-spends"))
					if err != nil || len(firstSpend) <= 112 {
						t.Fatal("actual initial Node spend absent", err)
					}
					budget, err := hosting.Open(budgetRoot)
					if err != nil {
						t.Fatal(err)
					}
					defer budget.Close()
					before, err := budget.Observe(t.Context())
					if err != nil || before.ReservedBytes != (64<<20)+(128<<10) {
						t.Fatal("original Node reservation", before, err)
					}
					var blocker *hosting.Reservation
					if mode == "capacity-before-spend" {
						// Leave space for real in-flight Carrier packets, then charge
						// actual loopback traffic through the provider's live counters.
						// A 501-byte snapshot margin can disappear before Reserve's
						// fresh measurement, especially with QUIC acknowledgement I/O.
						blocker, err = budget.Reserve(t.Context(), hosting.ReservationRequest{Work: hosting.Traffic{Tx: before.RemainingBytes - (64 << 10)}, Termination: hosting.Traffic{Tx: 1}, WorkUntil: end, HoldUntil: end.Add(time.Second)})
						if err != nil {
							t.Fatal("actual competing reservation", err)
						}
						defer releaseRouteReservation(blocker)
						chargeNativeLoopbackBudget(t)
						depleted, err := budget.Observe(t.Context())
						if err != nil || !depleted.Drain || depleted.UsedBytes <= before.UsedBytes {
							t.Fatal("actual provider traffic did not establish capacity loss", depleted, err)
						}
					}
					if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, second...)}); err != nil {
						t.Fatal(err)
					}
					ack, err = ardp.ReadFrame(conn)
					if mode == "refill" {
						if err != nil {
							t.Fatal("original Node refill failed", err)
						}
						status, credit, err = ardp.DecodeAcceptFrame(ack)
						if err != nil || status != 0 || credit != 65536 {
							t.Fatal("original Node refill refused", status, credit, err)
						}
						held, err := budget.Observe(t.Context())
						// The independently fixed wire debit is HELLO/ADMIT/ACCEPT
						// 617 bytes plus the later ADMIT's 371 bytes. No second
						// termination allowance or replacement deadline exists.
						if err != nil || held.ReservedBytes != before.ReservedBytes+988 {
							t.Fatal("original Node refill changed physical coverage", held, err)
						}
					} else if err == nil {
						status, _, decodeErr := ardp.DecodeAcceptFrame(ack)
						if decodeErr != nil || status == 0 {
							t.Fatal("real capacity refusal emitted acceptance", status, decodeErr)
						}
					}
					secondSpend, err := os.ReadFile(filepath.Join(spend, "closed-token-spends"))
					if err != nil {
						t.Fatal(err)
					}
					if mode == "refill" {
						if len(secondSpend) <= len(firstSpend) || !bytes.HasPrefix(secondSpend, firstSpend) {
							t.Fatal("accepted refill did not retain both real spends")
						}
					} else if !bytes.Equal(secondSpend, firstSpend) {
						t.Fatal("pre-spend capacity refusal burned the new token")
					}
					// Keep the real peer open until original Node cancellation.
					// A peer reset before stopping would manufacture collateral
					// physical failure in the positive cancellation control.
					process.stop(t, mode != "refill")
					if blocker != nil {
						if err := releaseRouteReservation(blocker); err != nil {
							t.Fatal(err)
						}
					}
					retained, err := os.ReadFile(filepath.Join(spend, "closed-token-spends"))
					if err != nil || !bytes.Equal(retained, secondSpend) {
						t.Fatal("joined command refunded or changed durable spend", err)
					}
					owner, err := receiving.Open(spend, f.receiver, func() (receiving.Observation, error) { return f.authority.receiver(f.receiver, member.NotAfter()) })
					if err != nil {
						t.Fatal("original spend root remains held after manager join", err)
					}
					defer owner.Close()
					reserve := func() (func() error, error) {
						held, err := budget.Reserve(context.Background(), hosting.ReservationRequest{Work: work, Termination: termination, WorkUntil: end, HoldUntil: end.Add(time.Second)})
						if err != nil {
							return nil, err
						}
						return func() error { return releaseRouteReservation(held) }, nil
					}
					if _, err := owner.Accept(t.Context(), admission.ForwardClass, first, end, reserve); err == nil || !strings.Contains(err.Error(), "closed token is already spent") {
						t.Fatal("original spend refunded on reopened Node root", err)
					}
					grant, err := owner.Accept(t.Context(), admission.ForwardClass, second, end, reserve)
					if mode == "refill" {
						if err == nil || !strings.Contains(err.Error(), "closed token is already spent") {
							t.Fatal("refill spend refunded after join/reopen", err)
						}
					} else {
						if err != nil {
							t.Fatal("pre-spend refusal consumed the fresh token", err)
						}
						if err := grant.Release(); err != nil {
							t.Fatal(err)
						}
					}
					final, err := budget.Observe(t.Context())
					if err != nil || final.ReservedBytes != 0 || final.UsedBytes < before.UsedBytes {
						t.Fatal("joined/reopened Node resource accounting", final, err)
					}
					if err := errors.Join(owner.Close(), budget.Close()); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}

func chargeNativeLoopbackBudget(t *testing.T) {
	t.Helper()
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	writer, err := net.Dial("udp", listener.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	end := time.Now().Add(time.Second)
	if err := errors.Join(listener.SetDeadline(end), writer.SetDeadline(end)); err != nil {
		t.Fatal(err)
	}
	payload, received := make([]byte, 2048), make([]byte, 2048)
	for range 40 {
		if n, err := writer.Write(payload); err != nil || n != len(payload) {
			t.Fatal("real provider-traffic write", n, err)
		}
		if n, _, err := listener.ReadFrom(received); err != nil || n != len(received) {
			t.Fatal("real provider-traffic receive", n, err)
		}
	}
}
