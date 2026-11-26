//go:build linux && node_process_native

package node_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/route/transport/tls"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// The driver owns only signed inputs and a real client. Both initial admission
// and parent refill execute inside the genuine original Node command, with its
// native process guard, producer, receiving journal and Hosting reservations.
func TestNativeNodeParentRefillAndReopenTCP(t *testing.T) {
	binary := processfixture.PinnedCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, mode := range []string{"refill", "capacity-before-spend"} {
				t.Run(mode, func(t *testing.T) {
					if remaining := time.Until(time.Now().UTC().Truncate(time.Hour).Add(time.Hour)); remaining < time.Minute {
						timer := time.NewTimer(remaining)
						defer timer.Stop()
						select {
						case <-timer.C:
						case <-t.Context().Done():
							t.Fatal(t.Context().Err())
						}
					}
					spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
					listener, err := net.Listen("tcp", "127.0.0.1:0")
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = listener.Close() })
					spec.Nodes[1].Endpoint = listener.Addr().String()
					bundle, err := networkfixture.BuildClosed(spec)
					if err != nil {
						t.Fatal(err)
					}
					profile := sha256.Sum256(bundle.Profile)
					networkinputs.PrepareIssuer(t, &issuerPlan, inventory, spec, bundle.Nodes[0], profile)
					clock, closeClock := networkinputs.StartObservedClock(t)
					defer closeClock()
					seed := func() state.AuthorityDeclaration {
						return networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)
					}
					authorityPlan := seed()
					config, err := authorityPlan.Config()
					if err != nil {
						t.Fatal(err)
					}
					authority, err := state.OpenRetained(config)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := authority.Close(); err != nil {
							t.Error(err)
						}
					})
					observe := func() (admission.AuthorityFacts, time.Time, error) {
						view, err := authority.CurrentRuntime()
						if err != nil {
							return admission.AuthorityFacts{}, time.Time{}, err
						}
						return funding.NetworkFacts(view)
					}
					facts, now, err := observe()
					if err != nil {
						t.Fatal(err)
					}
					id := bundle.Nodes[1].NodeID
					view, err := authority.CurrentRuntime()
					if err != nil {
						t.Fatal(err)
					}
					member, err := view.Member(id, view.ObservedAt())
					if err != nil {
						t.Fatal(err)
					}
					receiver := receiving.Receiver{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration,
						StateDigest: facts.StateDigest, ProfileDigest: facts.Digest, NodeID: id, DutyGeneration: member.DutyGeneration}
					guardPresentation := funding.PresentationGuard(authority.CurrentRuntime)
					holder := refillStock(t, spec.Authority, issuerPlan, receiver, facts, now, observe)
					key := bundle.Nodes[1].PrivateKey
					certPath, keyPath := processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, key))
					root, spend, budgetRoot := filepath.Join(t.TempDir(), "participation"), t.TempDir(), processfixture.ProcessBudget(t)
					work, termination := hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}
					plan := map[string]any{"network": seed(), "node_id": id, "process_profile": processresource.Profile,
						"participation_root": root, "spend_root": spend, "hosting_root": budgetRoot, "certificate": certPath,
						"private_key": keyPath, "work": work, "termination": termination, "retirement_wait_ms": 5000}
					planBytes, err := json.Marshal(plan)
					if err != nil {
						t.Fatal(err)
					}
					planPath := filepath.Join(t.TempDir(), "plan.json")
					if err := os.WriteFile(planPath, planBytes, 0600); err != nil {
						t.Fatal(err)
					}
					if err := listener.Close(); err != nil {
						t.Fatal(err)
					}
					process := processfixture.StartFreshRole(t, "forwarding", binary, planPath, 90)
					end := time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)
					conn, err := roletls.OpenEndpoint(t.Context(), transport.ClosedRoleCarrierRequest{CarrierProfile: carrier, Endpoint: member.Endpoint, ExpectedServer: member.PublicKey, Deadline: end})
					if err != nil {
						t.Fatal(err)
					}
					defer conn.Close()
					if err := conn.SetDeadline(end); err != nil {
						t.Fatal(err)
					}
					hello := ardp.Hello{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest, ProfileDigest: facts.Digest, RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration, Purpose: ardp.PurposeForwarding, Deadline: end}
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
						if err := guardPresentation(presentation); err != nil {
							t.Fatal(err)
						}
						raw, err := holder.Take(t.Context(), presentation, 2)
						if err != nil {
							t.Fatal(err)
						}
						if err := guardPresentation(presentation); err != nil {
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
						defer blocker.ReleaseJoined()
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
					process.Stop(t, mode != "refill")
					if blocker != nil {
						if err := blocker.ReleaseJoined(); err != nil {
							t.Fatal(err)
						}
					}
					retained, err := os.ReadFile(filepath.Join(spend, "closed-token-spends"))
					if err != nil || !bytes.Equal(retained, secondSpend) {
						t.Fatal("joined command refunded or changed durable spend", err)
					}
					owner, err := receiving.Open(spend, receiver, func() (receiving.Observation, error) {
						view, err := authority.CurrentRuntime()
						if err != nil {
							return receiving.Observation{}, err
						}
						current, now, err := funding.NetworkFacts(view)
						if err != nil {
							return receiving.Observation{}, err
						}
						active, err := view.Member(receiver.NodeID, now)
						if err != nil || active.DutyGeneration != receiver.DutyGeneration ||
							current.NetworkID != receiver.NetworkID || current.StateGeneration != receiver.StateGeneration ||
							current.StateDigest != receiver.StateDigest || current.Digest != receiver.ProfileDigest {
							return receiving.Observation{}, errors.Join(errors.New("original receiver duty unavailable"), err)
						}
						return receiving.Observation{Profile: current, Receiver: receiver, Now: now,
							NotAfter: earlierTime(earlierTime(member.NotAfter(), active.NotAfter()), current.NotAfter)}, nil
					})
					if err != nil {
						t.Fatal("original spend root remains held after manager join", err)
					}
					defer owner.Close()
					reserve := func() (func() error, error) {
						held, err := budget.Reserve(context.Background(), hosting.ReservationRequest{Work: work, Termination: termination, WorkUntil: end, HoldUntil: end.Add(time.Second)})
						if err != nil {
							return nil, err
						}
						return held.ReleaseJoined, nil
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

func refillStock(t *testing.T, authority ed25519.PrivateKey, plan issuer.Plan, receiver receiving.Receiver, facts admission.AuthorityFacts, now time.Time, observe func() (admission.AuthorityFacts, time.Time, error)) *stock.Owner {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	holder, err := stock.Open(root, admission.AllocationUser, observe)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := holder.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, digest, err := holder.Request([3]uint32{0, 2, 0})
	if err != nil {
		t.Fatal(err)
	}
	permission := processfixture.FixturePermission(t, raw, facts, authority, now)
	if err := holder.Import(digest, permission); err != nil {
		t.Fatal(err)
	}
	challenge := token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
		ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration, Class: uint8(admission.ForwardClass), WindowStart: now.UTC().Truncate(time.Hour)}
	attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{challenge, challenge},
		Selection: stock.ExchangeBinding{ID: [32]byte{31}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter})
	if err != nil {
		t.Fatal(err)
	}
	batch, _, err := attempt.Request()
	if err != nil {
		t.Fatal(err)
	}
	issued := issuer.IssueCurrent(t.Context(), plan, batch, quota.Bootstrap, observe)
	if issued.Outcome != "issued-offline" {
		t.Fatal("original fixture issuance", issued.Outcome)
	}
	if err := attempt.Complete(issued.Response, nil); err != nil {
		t.Fatal(err)
	}
	return holder
}

func earlierTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
