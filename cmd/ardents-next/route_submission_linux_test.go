//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// Genuine signed Network, Stock, receiving spend, Hosting, REGISTER and Source
// command composition carry unchanged opaque bytes on both Carriers. The input
// is deliberately unauthenticated and its holder replies refusal; this proves
// transport reachability, not a qualified Publisher or successful Connection.
func TestRouteSourceSubmissionRefusalBothCarriers(t *testing.T) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := publicationRouteNetwork(t, carrier)
			holder := routePermissionStock(t, f, admission.AllocationPublisher, [3]uint32{1, 8, 1})
			var batches [3][]token.ClosedTokenContext
			challenge := func(id byte, class uint8) token.ClosedTokenContext {
				return token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest,
					IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: [32]byte{id}, ReceiverDutyGeneration: 9,
					Class: class, WindowStart: time.Now().UTC().Truncate(time.Hour)}
			}
			for _, id := range []byte{12, 13, 14, 15, 21, 22, 23, 24} {
				batches[0] = append(batches[0], challenge(id, 2))
			}
			batches[1], batches[2] = []token.ClosedTokenContext{challenge(25, 3)}, []token.ClosedTokenContext{challenge(25, 1)}
			for i, contexts := range batches {
				bootstrap, kind := i < 2, quota.Admitted
				if bootstrap {
					kind = quota.Bootstrap
				}
				attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: contexts, Selection: stock.ExchangeBinding{ID: [32]byte{byte(60 + i)}, ProfileDigest: f.profile.Digest}, Bootstrap: bootstrap, Deadline: f.profile.NotAfter})
				if err != nil {
					t.Fatal(err)
				}
				request, _, err := attempt.Request()
				if err != nil {
					t.Fatal(err)
				}
				// Offline fixture issuance remains explicit, not a network issuer ACK.
				issued := issuer.IssueCurrent(t.Context(), f.plan, request, kind, func() (admission.AuthorityFacts, time.Time, error) {
					return f.authority.issuer(f.plan.KeyBinding.Signer)
				})
				if issued.Outcome != "issued-offline" {
					t.Fatal(issued.Outcome)
				}
				if err := attempt.Complete(issued.Response, nil); err != nil {
					t.Fatal(err)
				}
			}
			for _, id := range [][32]byte{{12}, {13}, {14}, {15}, {21}, {22}, {23}, {24}, {25}} {
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
				binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: id, DutyGeneration: 9}
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
				config := receiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id]}
				if id == [32]byte{25} {
					config.IntroductionRoot, config.Receiving = t.TempDir(), owner
					if err := os.Chmod(config.IntroductionRoot, 0700); err != nil {
						t.Fatal(err)
					}
				}
				config.Admit = func(ctx context.Context, channel receiver.Channel, raw []byte) (receiving.Grant, error) {
					class, err := role.AdmissionClass(channel.Hello.Purpose)
					if err != nil {
						return receiving.Grant{}, err
					}
					return owner.Accept(ctx, class, raw, channel.Hello.Deadline, func() (func() error, error) {
						release, err := networkTestReservation(t, budget, channel.Hello.Deadline)
						if err != nil {
							return nil, err
						}
						return channel.HoldReservation(release)
					})
				}
				sockets[id]()
				server, err := receiver.Listen(t.Context(), config)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := server.Close(); err != nil {
						t.Errorf("receiver %d retirement: %v", id[0], err)
					}
				})
			}
			plan := routePrefixPlan{Deadline: time.Now().Add(40 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			root := t.TempDir()
			selected, err := selection.Open(selection.Config{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), Domain: 4, Current: f.authority.current})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := selected.Close(); err != nil {
					t.Error(err)
				}
			}()
			budget, err := hosting.Open(routeProcessBudget(t))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := budget.Close(); err != nil {
					t.Error(err)
				}
			}()
			intro, leg, err := openPublicationPrefix(t.Context(), plan, selected, budget, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := intro.Close(); err != nil {
					t.Error(err)
				}
			}()
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			duty, err := leg.IntroductionDuty(view, nil)
			if err != nil {
				t.Fatal(err)
			}
			registration, err := introduction.Register(t.Context(), intro, introduction.RegistrationConfig{Duty: duty, Revision: 1, Deadline: plan.Deadline})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := registration.Close(); err != nil {
					t.Error(err)
				}
			}()
			sourceRoot := t.TempDir()
			sourcePlan := plan
			sourcePlan.EntryRoot, sourcePlan.InteriorRoot, sourcePlan.HostingRoot, sourcePlan.Domain = filepath.Join(sourceRoot, "entry"), filepath.Join(sourceRoot, "interior"), routeProcessBudget(t), 1
			source, err := startRoutePrefix(t.Context(), sourcePlan, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := source.close(); err != nil {
					t.Error(err)
				}
			}()
			receipt, err := registration.Receipt()
			if err != nil {
				t.Fatal(err)
			}
			raw := make([]byte, 474)
			facts := receipt.Facts()
			copy(raw[:32], facts.Slot[:])
			binary.BigEndian.PutUint64(raw[32:40], 1)
			binary.BigEndian.PutUint64(raw[40:48], uint64(time.Now().Add(8*time.Second).Unix()))
			raw[48], raw[80], raw[114] = 30, 40, 50
			binary.BigEndian.PutUint16(raw[112:114], 360)
			if _, err := capsule.Parse(raw); err != nil {
				t.Fatal(err)
			}
			replied := make(chan error, 1)
			go func() {
				child, err := registration.NextDelivery(t.Context())
				if err == nil {
					if !bytes.Equal(child.Capsule().Bytes(), raw) {
						err = errors.New("Introduction changed opaque capsule")
					} else {
						err = child.Reply(child.Context(), 1)
					}
					child.Close()
				}
				replied <- err
			}()
			err = source.submitCapsule(t.Context(), routeRecipient{Node: duty.NodeID, Generation: duty.RecordGeneration}, raw)
			if !submissionRefusalOnly(err) {
				t.Fatal("genuine exact refusal/physical join differs", err)
			}
			if err := <-replied; err != nil {
				t.Fatal("recipient refusal did not join matching CLOSE", err)
			}
			if err := registration.Withdraw(t.Context()); err != nil {
				t.Fatal("owning withdrawal after delivery", err)
			}
		})
	}
}

func submissionRefusalOnly(err error) bool {
	if err == nil {
		return false
	}
	if refusal, ok := err.(introduction.SubmissionRefusal); ok {
		return refusal.Status == 1
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if !submissionRefusalOnly(child) {
				return false
			}
		}
		return len(joined.Unwrap()) > 0
	}
	return false
}
