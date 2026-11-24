//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/hpke"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The root driver supplies only independently signed inputs and a bounded
// isolated HPKE holder recipient, not a qualified Job/Instance/Service. Actual
// signed State, issuing Stock, new Forwarding and the canonical native Node
// Introduction command supply every authority, spend, reservation and ACK.
func TestNativeIntroductionRegisterSubmitWithdrawTCP(t *testing.T) {
	nativeIntroductionCommandWorkload(t, "withdraw")
}
func TestNativeIntroductionUndercoverageBeforeSpendTCP(t *testing.T) {
	nativeIntroductionCommandWorkload(t, "undercoverage")
}
func TestNativeIntroductionOriginalHistoryLossTCP(t *testing.T) {
	nativeIntroductionCommandWorkload(t, "history-loss")
}

func TestNativeIntroductionOriginalCommandFailureTCP(t *testing.T) {
	for _, fault := range []string{"protection-loss", "clock-loss"} {
		t.Run(fault, func(t *testing.T) { nativeIntroductionCommandWorkload(t, fault) })
	}
}

func nativeIntroductionCommandWorkload(t *testing.T, mode string) {
	commandBinary := processfixture.PinnedCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := publicationRouteNetwork(t, carrier)
			holder := nativeIntroductionStock(t, f)

			for _, id := range [][32]byte{{12}, {13}, {14}, {15}, {21}, {22}, {23}, {24}} {
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

				config.Admit = func(ctx context.Context, channel receiver.Channel, raw []byte) (receiving.Grant, error) {
					class, err := role.AdmissionClass(channel.Hello.Purpose)
					if err != nil {
						return receiving.Grant{}, err
					}
					return owner.Accept(ctx, class, raw, channel.Hello.Deadline, func() (func() error, error) {
						limit := class.ByteLimit()
						held, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: hosting.Traffic{Tx: limit, Rx: limit}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: channel.Hello.Deadline, HoldUntil: channel.Hello.Deadline.Add(5 * time.Second)})
						if err == nil {
							err = budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit})
						}
						release := held.ReleaseJoined
						if err != nil {
							return nil, errors.Join(err, release())
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
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := processfixture.StartObservedClock(t)
			var clockOnce sync.Once
			stopClock := func() { clockOnce.Do(closeClock) }
			defer stopClock()
			cert, key := processfixture.WriteNativeCredential(t, certificates[[32]byte{25}])
			historyRoot, spendRoot := t.TempDir(), t.TempDir()
			if err := os.Chmod(historyRoot, 0700); err != nil {
				t.Fatal(err)
			}
			nodePlan := map[string]any{"network": processfixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock), "node_id": [32]byte{25}, "process_profile": processresource.IntroductionProfile,
				"participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": spendRoot, "hosting_root": processfixture.ProcessBudget(t), "introduction_root": historyRoot,
				"certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
			if mode == "undercoverage" {
				nodePlan["work"] = hosting.Traffic{Tx: (1 << 20) - 1}
			}
			sockets[[32]byte{25}]()
			process := processfixture.StartFreshRole(t, "introduction", commandBinary, hostingConfig(t, nodePlan), 25)
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
			budget, err := hosting.Open(processfixture.ProcessBudget(t))
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
			initialSpend, readErr := os.ReadFile(filepath.Join(spendRoot, "closed-token-spends"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			initialHistory, readErr := os.ReadFile(filepath.Join(historyRoot, "closed-introduction-slots"))
			if readErr != nil || len(initialHistory) != 120 {
				t.Fatal("genuine initial history differs", readErr)
			}
			registration, err := introduction.Register(t.Context(), intro, introduction.RegistrationConfig{Duty: duty, Revision: 1, Deadline: plan.Deadline})
			if mode == "undercoverage" {
				if err == nil {
					_ = registration.Close()
					t.Fatal("undercovered complete Registration class admitted")
				}
				afterSpend, readErr := os.ReadFile(filepath.Join(spendRoot, "closed-token-spends"))
				afterHistory, historyErr := os.ReadFile(filepath.Join(historyRoot, "closed-introduction-slots"))
				if readErr != nil || historyErr != nil || !bytes.Equal(afterSpend, initialSpend) || !bytes.Equal(afterHistory, initialHistory) {
					t.Fatal("pre-spend undercoverage changed spend/history", readErr, historyErr)
				}
				_ = intro.Close() // failed child retains its true physical result
				process.Stop(t, true)
				return
			}
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
			sourcePlan.EntryRoot, sourcePlan.InteriorRoot, sourcePlan.HostingRoot, sourcePlan.Domain = filepath.Join(sourceRoot, "entry"), filepath.Join(sourceRoot, "interior"), processfixture.ProcessBudget(t), 1
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
			facts := receipt.Facts()
			private, err := ecdh.X25519().GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			var public [32]byte
			copy(public[:], private.PublicKey().Bytes())
			expiry := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
			request := capsule.Request{Network: f.profile.NetworkID, ProfileDigest: f.profile.Digest, Target: [32]byte{1}, PublicationDigest: [32]byte{2}, Revision: facts.Revision,
				RendezvousNode: [32]byte{3}, RendezvousDutyGeneration: 9, JoinSecret: [32]byte{4}, HandshakeContext: [32]byte{5}, ConnectionNonce: [32]byte{6}, AttachmentGeneration: 1,
				Deadline: expiry, InitiatorBinding: [32]byte{7}, WorkSafetyNotAfter: expiry.Unix(), WorkSafetyMaximum: expiry.Unix(), NoNewRecoveryAfter: expiry.Unix()}
			envelope, _, err := capsule.Seal(capsule.Header{Slot: facts.Slot, Revision: facts.Revision, Expiry: expiry, DeliveryNonce: [32]byte{8}}, public, request)
			if err != nil {
				t.Fatal(err)
			}
			raw := envelope.Bytes()
			replied := make(chan error, 1)
			go func() {
				child, err := registration.NextDelivery(t.Context())
				if err == nil {
					if !bytes.Equal(child.Capsule().Bytes(), raw) {
						err = errors.New("Introduction changed opaque capsule")
					} else {
						key, openErr := hpke.NewDHKEMPrivateKey(private)
						info := append([]byte("ardents-introduction-capsule-v3\x00"), f.profile.Digest[:]...)
						if openErr == nil {
							recipient, recipientErr := hpke.NewRecipient(raw[80:112], key, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
							if recipientErr != nil {
								openErr = recipientErr
							} else {
								plain, plainErr := recipient.Open(append(append([]byte(nil), info...), raw[:80]...), raw[114:])
								if plainErr != nil {
									openErr = plainErr
								} else {
									got, parseErr := capsule.ParseRequest(plain)
									clear(plain)
									if parseErr != nil {
										openErr = parseErr
									} else if got != request {
										openErr = errors.New("isolated recipient plaintext differs")
									}
								}
							}
						}
						if openErr != nil {
							err = openErr
						} else {
							err = child.Reply(child.Context(), 0)
						}
					}
					child.Close()
				}
				replied <- err
			}()
			err = source.submitCapsule(t.Context(), routeRecipient{Node: duty.NodeID, Generation: duty.RecordGeneration}, raw)
			if err != nil {
				t.Fatal("genuine opaque delivery/physical join differs", err)
			}
			if err := <-replied; err != nil {
				t.Fatal("isolated recipient RESULT did not join matching CLOSE", err)
			}
			if err := registration.Withdraw(t.Context()); err != nil {
				t.Fatal("owning withdrawal after delivery", err)
			}
			if err := registration.Close(); err != nil {
				t.Fatal(err)
			}
			if err := source.close(); err != nil {
				t.Fatal(err)
			}
			if err := intro.Close(); err != nil {
				t.Fatal(err)
			}
			if mode == "history-loss" {
				path := filepath.Join(historyRoot, "closed-introduction-slots")
				before, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				process.FailedJoin(t, "introduction")
				// Restoration is only evidence for a separate full reopen; the
				// failed original has physically joined and cannot revive.
				if err := os.WriteFile(path, before, 0600); err != nil {
					t.Fatal(err)
				}
			} else if mode == "protection-loss" {
				if err := exec.Command("systemctl", "set-property", "--runtime", process.Unit, "CPUQuota=50%").Run(); err != nil {
					t.Fatal(err)
				}
				process.FailedJoin(t, "process")
			} else if mode == "clock-loss" {
				stopClock()
				if err := os.Remove(clock); err != nil {
					t.Fatal(err)
				}
				process.FailedJoin(t, "network")
			} else {
				process.Stop(t)
			}
			retained, err := introduction.OpenHistory(historyRoot, introduction.Binding{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, ReceiverNodeID: [32]byte{25}, ReceiverDutyGeneration: 9})
			if err != nil {
				t.Fatal(err)
			}
			defer retained.Close()
			if err := retained.Claim(facts.Slot, facts.Expiry, time.Now().UTC().Truncate(time.Second)); err == nil {
				t.Fatal("owning withdrawal/native command join reclaimed original slot")
			}
			snapshot, err := os.ReadFile(filepath.Join(historyRoot, "closed-introduction-slots"))
			if err != nil || len(snapshot) != 160 || string(snapshot[:8]) != "ARDISL01" || binary.BigEndian.Uint64(snapshot[152:160]) != uint64(facts.Expiry.Unix()) {
				t.Fatal("original durable floor differs", err)
			}

		})
	}
}
func nativeIntroductionStock(t *testing.T, f *networkAdmissionFixture) *stock.Owner {
	t.Helper()
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
	return holder
}
