//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/hpke"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
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
	commandBinary := nodeNativeCommand(t)
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
						release := func() error { return releaseRouteReservation(held) }
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
			clock, closeClock := joinProcessClock(t)
			var clockOnce sync.Once
			stopClock := func() { clockOnce.Do(closeClock) }
			defer stopClock()
			cert, key := nativeDescriptorCredential(t, certificates[[32]byte{25}])
			historyRoot, spendRoot := t.TempDir(), t.TempDir()
			if err := os.Chmod(historyRoot, 0700); err != nil {
				t.Fatal(err)
			}
			nodePlan := map[string]any{"network": seedNativeNodeState(t, f, bundle, clock), "node_id": [32]byte{25}, "process_profile": processresource.IntroductionProfile,
				"participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": spendRoot, "hosting_root": routeProcessBudget(t), "introduction_root": historyRoot,
				"certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
			if mode == "undercoverage" {
				nodePlan["work"] = hosting.Traffic{Tx: (1 << 20) - 1}
			}
			sockets[[32]byte{25}]()
			process := startNativeIntroduction(t, commandBinary, hostingConfig(t, nodePlan), 25)
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
				process.stop(t, true)
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
				process.failedJoin(t, "introduction")
				// Restoration is only evidence for a separate full reopen; the
				// failed original has physically joined and cannot revive.
				if err := os.WriteFile(path, before, 0600); err != nil {
					t.Fatal(err)
				}
			} else if mode == "protection-loss" {
				if err := exec.Command("systemctl", "set-property", "--runtime", process.unit, "CPUQuota=50%").Run(); err != nil {
					t.Fatal(err)
				}
				process.failedJoin(t, "process")
			} else if mode == "clock-loss" {
				stopClock()
				if err := os.Remove(clock); err != nil {
					t.Fatal(err)
				}
				process.failedJoin(t, "network")
			} else {
				process.stop(t)
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
func launchNativeIntroduction(t *testing.T, binary, plan string, index byte) *nativeForwardingProcess {
	t.Helper()
	unit := fmt.Sprintf("ardents-introduction-%d-%d.service", os.Getpid(), index)
	p := &nativeForwardingProcess{unit: unit}
	p.command = exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", binary, "node", "introduction", "--config", plan)
	p.command.Stderr = &p.diagnostic
	var err error
	p.stdout, err = p.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !p.joined {
			_ = exec.Command("systemctl", "stop", unit).Run()
			_ = p.command.Wait()
			_ = p.stdout.Close()
			p.joined = true
		}
		if t.Failed() {
			t.Log("original Introduction retained diagnostic", p.diagnostic.String())
		}
	})
	return p
}

func startNativeIntroduction(t *testing.T, binary, plan string, index byte) *nativeForwardingProcess {
	t.Helper()
	p := launchNativeIntroduction(t, binary, plan, index)
	var event map[string]string
	if err := json.NewDecoder(p.stdout).Decode(&event); err != nil || event["operation"] != "node.introduction" || event["phase"] != "listening" {
		t.Fatal("genuine original Introduction startup refused", event, err, p.diagnostic.String())
	}
	raw, err := exec.Command("systemctl", "show", p.unit, "-p", "MainPID", "--value").Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Sscanf(string(raw), "%d", &p.pid); err != nil || p.pid < 2 {
		t.Fatal("original Introduction MainPID unavailable", err)
	}
	return p
}

func TestNativeIntroductionPartialStartupRefusalsTCP(t *testing.T) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, fault := range []string{"profile", "unknown-field", "nested-roots", "role", "tls-key", "missing-hosting", "foreign-history", "foreign-spend"} {
				t.Run(fault, func(t *testing.T) {
					f, sockets, certificates := publicationRouteNetwork(t, carrier)
					bundle, err := networkfixture.BuildClosed(f.spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := joinProcessClock(t)
					defer closeClock()
					id := [32]byte{25}
					cert, key := nativeDescriptorCredential(t, certificates[id])
					participation, spend, budget, historyRoot := filepath.Join(t.TempDir(), "participation"), t.TempDir(), routeProcessBudget(t), t.TempDir()
					plan := map[string]any{"network": seedNativeNodeState(t, f, bundle, clock), "node_id": id, "process_profile": processresource.IntroductionProfile,
						"participation_root": participation, "spend_root": spend, "hosting_root": budget, "introduction_root": historyRoot,
						"certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
					if err := os.Chmod(historyRoot, 0700); err != nil {
						t.Fatal(err)
					}
					expected := 1
					var foreign string
					switch fault {
					case "profile":
						plan["process_profile"], expected = processresource.Profile, 2
					case "unknown-field":
						plan["accepting"] = true
						expected = 2
					case "nested-roots":
						plan["introduction_root"], expected = filepath.Join(spend, "nested"), 2
					case "role":
						plan["node_id"] = [32]byte{12}
					case "tls-key":
						plan["certificate"], plan["private_key"] = nativeDescriptorCredential(t, certificates[[32]byte{12}])
					case "missing-hosting":
						plan["hosting_root"] = filepath.Join(t.TempDir(), "missing")
					case "foreign-history":
						foreign = filepath.Join(historyRoot, "closed-introduction-slots")
					case "foreign-spend":
						foreign = filepath.Join(spend, "foreign-input")
					}
					foreignBytes := []byte("independent rejected input\n")
					if foreign != "" {
						if err := os.WriteFile(foreign, foreignBytes, 0600); err != nil {
							t.Fatal(err)
						}
					}
					sockets[id]()
					p := launchNativeIntroduction(t, binary, hostingConfig(t, plan), id[0])
					var event map[string]string
					decodeErr := json.NewDecoder(p.stdout).Decode(&event)
					err = p.command.Wait()
					p.joined = true
					_ = p.stdout.Close()
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != expected {
						t.Fatal("actual Introduction startup did not refuse", err, p.diagnostic.String())
					}
					if expected == 1 && (decodeErr != nil || len(event) != 3 || event["operation"] != "node.introduction" || event["phase"] != "refused" || event["outcome"] != "startup-unavailable") {
						t.Fatal("partial startup lost bounded refusal", event, decodeErr)
					}
					if expected == 2 && (decodeErr == nil || len(event) != 0) {
						t.Fatal("invalid declaration reached runtime", event)
					}
					spendBytes, spendErr := os.ReadFile(filepath.Join(spend, "closed-token-spends"))
					if fault == "foreign-history" {
						// Receiving necessarily precedes Route birth-fact consumption. This is
						// exactly its empty canonical header, never a spend or permission.
						if spendErr != nil || len(spendBytes) != 112 || string(spendBytes[:8]) != "ARDSPN01" {
							t.Fatal("failed Route startup spent or damaged Receiving", spendErr)
						}
					} else if !os.IsNotExist(spendErr) {
						t.Fatal("earlier refusal created receiving history", spendErr)
					}
					if foreign != "" {
						raw, err := os.ReadFile(foreign)
						if err != nil || !bytes.Equal(raw, foreignBytes) {
							t.Fatal("rejected private input was rewritten", err)
						}
						// Remove only this test's unaccepted input, never History records.
						if err := os.Remove(foreign); err != nil {
							t.Fatal(err)
						}
					}
					if _, err := os.Stat(participation); err == nil {
						view, err := f.current()
						if err != nil {
							t.Fatal(err)
						}
						member, err := view.Member(id, view.ObservedAt())
						if err != nil {
							t.Fatal(err)
						}
						journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation, Clock: time.Now})
						if err != nil {
							t.Fatal("partial startup participation not released", err)
						}
						conflict, err := journal.Conflict(id, member.FamilyID)
						closeErr := journal.Close()
						if err != nil || closeErr != nil || conflict {
							t.Fatal("partial startup retained original producer", err, closeErr, conflict)
						}
					}
					delete(plan, "accepting")
					plan["process_profile"], plan["node_id"] = processresource.IntroductionProfile, id
					plan["certificate"], plan["private_key"] = cert, key
					plan["hosting_root"], plan["introduction_root"] = budget, historyRoot
					if fault == "foreign-history" {
						retained := launchNativeIntroduction(t, binary, hostingConfig(t, plan), id[0])
						var refusal map[string]string
						if err := json.NewDecoder(retained.stdout).Decode(&refusal); err != nil || refusal["phase"] != "refused" {
							t.Fatal("retained empty receiving root recreated birth fact", refusal, err)
						}
						joined := retained.command.Wait()
						retained.joined = true
						_ = retained.stdout.Close()
						var exit *exec.ExitError
						if !errors.As(joined, &exit) || exit.ExitCode() != 1 {
							t.Fatal("retained root admitted after rejected History", joined)
						}
						plan["spend_root"], plan["introduction_root"] = t.TempDir(), t.TempDir()
						if err := os.Chmod(plan["introduction_root"].(string), 0700); err != nil {
							t.Fatal(err)
						}
					}
					fresh := startNativeIntroduction(t, binary, hostingConfig(t, plan), id[0])
					fresh.stop(t)
				})
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
