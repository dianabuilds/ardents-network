//go:build linux && node_process_native

package node_test

import (
	"bytes"
	"crypto/ecdh"
	"crypto/hpke"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/route/introduction"
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
	for _, carrier := range []string{epoch.CarrierClosedTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			processfixture.WaitAdmissionHour(t)
			spec, issuerPlan, inventory := processfixture.PrepareNetworkGenesis(t, t.TempDir())
			spec.Nodes = spec.Nodes[:1]
			listeners := make(map[[32]byte]net.Listener)
			certificates := make(map[[32]byte]tls.Certificate)
			for _, id := range []byte{12, 13, 14, 15, 21, 22, 23, 24, 25, 17} {
				domain, subrole := byte(1), byte(1)
				switch {
				case id == 17:
					domain, subrole = 2, 5
				case id == 25:
					domain, subrole = 4, 3
				case id >= 23:
					domain, subrole = 4, 2
				case id >= 21:
					domain = 4
				case id >= 14:
					subrole = 2
				}
				processfixture.AddReceivingFixtureNode(t, &spec, listeners, certificates, carrier, id, domain, subrole)
			}
			bundle, err := networkfixture.BuildClosed(spec)
			if err != nil {
				t.Fatal(err)
			}
			processfixture.PrepareIssuer(t, &issuerPlan, inventory, spec, bundle.Nodes[0], sha256.Sum256(bundle.Profile))
			networkClock, closeNetworkClock := processfixture.StartObservedClock(t)
			defer closeNetworkClock()
			clock, closeClock := processfixture.StartObservedClock(t)
			stopClock := sync.OnceFunc(closeClock)
			defer stopClock()
			seed := func(clock string) state.AuthorityDeclaration {
				return processfixture.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)
			}
			declaration := seed(networkClock)
			config, err := declaration.Config()
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
			view, err := authority.CurrentRuntime()
			if err != nil {
				t.Fatal(err)
			}
			profile, _, err := funding.NetworkFacts(view)
			if err != nil {
				t.Fatal(err)
			}
			holder := processfixture.IntroductionStock(t, spec, issuerPlan, authority.CurrentRuntime)
			var forwarding []*processfixture.NativeProcess
			for _, id := range []byte{12, 13, 14, 15, 21, 22, 23, 24} {
				nodeID := [32]byte{id}
				cert, key := processfixture.WriteNativeCredential(t, certificates[nodeID])
				plan := map[string]any{"network": seed(networkClock), "node_id": nodeID, "process_profile": processresource.Profile,
					"participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": t.TempDir(),
					"hosting_root": processfixture.ProcessBudget(t), "certificate": cert, "private_key": key,
					"work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
				if err := listeners[nodeID].Close(); err != nil {
					t.Fatal(err)
				}
				forwarding = append(forwarding, processfixture.StartFreshRole(t, "forwarding", commandBinary, writeAuthorityProcessPlan(t, plan), id))
			}
			cert, key := processfixture.WriteNativeCredential(t, certificates[[32]byte{25}])
			historyRoot, spendRoot := t.TempDir(), t.TempDir()
			if err := os.Chmod(historyRoot, 0700); err != nil {
				t.Fatal(err)
			}
			nodePlan := map[string]any{"network": seed(clock), "node_id": [32]byte{25}, "process_profile": processresource.IntroductionProfile,
				"participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": spendRoot, "hosting_root": processfixture.ProcessBudget(t), "introduction_root": historyRoot,
				"certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
			if mode == "undercoverage" {
				nodePlan["work"] = hosting.Traffic{Tx: (1 << 20) - 1}
			}
			if err := listeners[[32]byte{25}].Close(); err != nil {
				t.Fatal(err)
			}
			process := processfixture.StartFreshRole(t, "introduction", commandBinary, writeAuthorityProcessPlan(t, nodePlan), 25)
			root := t.TempDir()
			plan := funding.Plan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: processfixture.ProcessBudget(t),
				Domain: 4, Deadline: time.Now().Add(40 * time.Second).UTC().Truncate(time.Second),
				Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			routeAuthority := funding.Observe(authority.CurrentRuntime, authority.CurrentRuntimeWitness, funding.PresentationGuard(authority.CurrentRuntime))
			intro, err := session.OpenPrefix(t.Context(), plan, routeAuthority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := intro.Close(); err != nil && mode != "undercoverage" {
					t.Error(err)
				}
			}()
			initialSpend, readErr := os.ReadFile(filepath.Join(spendRoot, "closed-token-spends"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			initialHistory, readErr := os.ReadFile(filepath.Join(historyRoot, "closed-introduction-slots"))
			if readErr != nil || len(initialHistory) != 120 {
				t.Fatal("genuine initial history differs", readErr)
			}
			registered, err := intro.Register(t.Context(), 1)
			if mode == "undercoverage" {
				if err == nil {
					t.Log("unexpected registration cleanup", registered.Owner().Close())
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
			registration, ok := registered.Owner().(*introduction.HolderRegistration)
			if !ok {
				t.Fatal("registration lacks its original socket owner")
			}
			defer func() {
				if err := registration.Close(); err != nil {
					t.Error(err)
				}
			}()
			sourceRoot := t.TempDir()
			sourcePlan := plan
			sourcePlan.EntryRoot, sourcePlan.InteriorRoot, sourcePlan.HostingRoot, sourcePlan.Domain = filepath.Join(sourceRoot, "entry"), filepath.Join(sourceRoot, "interior"), processfixture.ProcessBudget(t), 1
			source, err := session.OpenPrefix(t.Context(), sourcePlan, routeAuthority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := source.Close(); err != nil {
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
			request := capsule.Request{Network: profile.NetworkID, ProfileDigest: profile.Digest, Target: [32]byte{1}, PublicationDigest: [32]byte{2}, Revision: facts.Revision,
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
						info := append([]byte("ardents-introduction-capsule-v3\x00"), profile.Digest[:]...)
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
			err = source.SubmitCapsule(t.Context(), session.Recipient{Node: facts.Node, Generation: 9}, raw)
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
			if err := source.Close(); err != nil {
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
			retained, err := introduction.OpenHistory(historyRoot, introduction.Binding{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, ReceiverNodeID: [32]byte{25}, ReceiverDutyGeneration: 9})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := retained.Close(); err != nil {
					t.Error(err)
				}
			}()
			for _, peer := range forwarding {
				peer.Stop(t)
			}
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
