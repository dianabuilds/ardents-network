//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	routeprefix "github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	routereceiver "github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Native profile: genuine signed State, durable Admission/Hosting/Store owners
// and physical selected Carriers. Public signed input is not a live Publisher,
// registered slot, Execution Job or authenticated Service Connection.
func TestRouteGenuineDescriptorBothCarriers(t *testing.T) {
	t.Run("Introduction-refusals", func(t *testing.T) { testRouteGenuineDescriptor(t, false) })
	t.Run("profile-refusal", func(t *testing.T) { testRouteGenuineDescriptor(t, true) })
}

func testRouteGenuineDescriptor(t *testing.T, foreignProfileControl bool) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			f, reservations, certificates := descriptorRouteFixture(t, carrier)
			holder := descriptorRouteStock(t, f)
			store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), Network: f.profile.NetworkID})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := store.Close(); err != nil {
					t.Error(err)
				}
			})
			var controlSpends atomic.Int32
			for _, id := range [][32]byte{{12}, {13}, {14}, {15}, {17}} {
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
				config := routereceiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id]}
				if id == [32]byte{17} {
					config.DescriptorStore = store
				}
				config.Admit = func(ctx context.Context, channel routereceiver.Channel, raw []byte) (receiving.Grant, error) {
					class, err := role.AdmissionClass(channel.Hello.Purpose)
					if err != nil {
						return receiving.Grant{}, err
					}
					grant, err := owner.Accept(ctx, class, raw, channel.Hello.Deadline, func() (func() error, error) {
						release, err := networkTestReservation(t, budget, channel.Hello.Deadline)
						if err != nil {
							return nil, err
						}
						return channel.HoldReservation(release)
					})
					if err == nil && class == admission.ControlClass {
						controlSpends.Add(1)
					}
					return grant, err
				}
				reservations[id]()
				server, err := routereceiver.Listen(t.Context(), config)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := server.Close(); err != nil {
						t.Error("receiving join", err)
					}
				})
			}
			root := t.TempDir()
			plan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			prefix, err := startRoutePrefix(t.Context(), plan, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := prefix.close(); err != nil {
					t.Error("holder join", err)
				}
			}()
			history := &reachability.History{}
			defer func() { _ = history.Close() }()
			raw, target := routeSignedDescriptor(t, f, 3, [32]byte{18})
			if err := prefix.publishDescriptor(t.Context(), raw); err != nil {
				t.Fatal("actual Store publication", err)
			}
			got, err := prefix.lookupDescriptor(t.Context(), target, history)
			if err != nil || !bytes.Equal(got, raw) {
				t.Fatal("independent fresh lookup differs", err)
			}
			connectOnly, _ := routeSignedDescriptor(t, f, 2, [32]byte{18})
			if err := prefix.publishDescriptor(t.Context(), connectOnly); err == nil {
				t.Fatal("Connect-only published through actual receiving Store")
			} else {
				var refused routeprefix.DescriptorRefusal
				if !errors.As(err, &refused) || refused.Status != 1 {
					t.Fatal("missing capability did not retain fixed refusal", err)
				}
			}
			broken := bytes.Clone(raw)
			broken[len(broken)-1] ^= 1
			if err := prefix.publishDescriptor(t.Context(), broken); err == nil {
				t.Fatal("invalid Instance signature accepted")
			}
			if foreignProfileControl {
				foreignProfile := f.profile.Digest
				foreignProfile[0] ^= 1
				wrongProfile, _ := routeSignedDescriptorProfile(t, f, 3, [32]byte{18}, foreignProfile)
				if _, err := reachability.VerifyPublish(wrongProfile, f.profile.NetworkID, foreignProfile, time.Now()); err != nil {
					t.Fatal("foreign-profile control has invalid signatures", err)
				}
				if err := prefix.publishDescriptor(t.Context(), wrongProfile); err == nil {
					t.Fatal("signed foreign-profile Descriptor accepted by actual receiver")
				} else {
					var refused routeprefix.DescriptorRefusal
					if !errors.As(err, &refused) || refused.Status != 1 {
						t.Fatal("foreign profile did not retain fixed refusal", err)
					}
				}
			}
			for _, introduction := range [][32]byte{{12}, {99}} {
				if foreignProfileControl && introduction == [32]byte{99} {
					continue
				}
				invalidDuty, _ := routeSignedDescriptor(t, f, 3, introduction)
				if err := prefix.publishDescriptor(t.Context(), invalidDuty); err == nil {
					t.Fatal("signed Descriptor with non-Introduction or absent duty accepted")
				}
			}
			got, err = prefix.lookupDescriptor(t.Context(), target, history)
			if err != nil || !bytes.Equal(got, raw) {
				t.Fatal("refused publication changed retained proof", err)
			}
			if _, err := prefix.lookupDescriptor(t.Context(), [32]byte{99}, history); err == nil {
				t.Fatal("lookup changed independently selected Target")
			}
			if controlSpends.Load() != 8 {
				t.Fatal("fresh terminal admission not spent for every operation", controlSpends.Load())
			}
		})
	}
}

func descriptorRouteFixture(t *testing.T, carrier transport.CarrierProfile) (*networkAdmissionFixture, map[[32]byte]func(), map[[32]byte]tls.Certificate) {
	return newRoleRouteFixtureConfigured(t, carrier, 1, false, false, func(f *networkAdmissionFixture, reservations map[[32]byte]func(), certificates map[[32]byte]tls.Certificate) {
		for _, id := range [][32]byte{{17}, {18}} {
			_, key, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { clear(key) })
			address := "127.0.0.1:1"
			if id == [32]byte{17} {
				if carrier == transport.ClosedCarrierTCP {
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
			}
			domain, subrole := byte(2), byte(5)
			if id == [32]byte{18} {
				domain, subrole = 4, 3
			}
			f.spec.Nodes = append(f.spec.Nodes, networkfixture.ClosedNode{RecordSpec: networkfixture.RecordSpec{NodeID: id, Generation: 9, ValidFrom: f.spec.NotBefore, ValidUntil: f.spec.NotAfter, Endpoint: address, Carrier: string(carrier), Capability: 2, Capacity: 1, PrivateKey: key}, RoleDomain: domain, Subrole: subrole})
			certificates[id] = routeTestCertificate(t, key)
		}
	})
}

func descriptorRouteStock(t *testing.T, f *networkAdmissionFixture) *stock.Owner {
	holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{8, 4, 0})
	for i, challenges := range descriptorRouteChallenges(f) {
		attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{byte(41 + i)}, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: f.profile.NotAfter})
		if err != nil {
			t.Fatal(err)
		}
		batch, _, err := attempt.Request()
		if err != nil {
			t.Fatal(err)
		}
		issued := issuer.IssueCurrent(t.Context(), f.plan, batch, quota.Bootstrap, func() (admission.AuthorityFacts, time.Time, error) {
			return f.authority.issuer(f.plan.KeyBinding.Signer)
		})
		if issued.Outcome != "issued-offline" {
			t.Fatal(issued)
		}
		if err := attempt.Complete(issued.Response, nil); err != nil {
			t.Fatal(err)
		}
	}
	return holder
}

func descriptorRouteChallenges(f *networkAdmissionFixture) [][]token.ClosedTokenContext {
	var forwards, controls []token.ClosedTokenContext
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}} {
		forwards = append(forwards, token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: id, ReceiverDutyGeneration: 9, Class: 2, WindowStart: time.Now().UTC().Truncate(time.Hour)})
	}
	for range 8 {
		controls = append(controls, token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: [32]byte{17}, ReceiverDutyGeneration: 9, Class: 1, WindowStart: time.Now().UTC().Truncate(time.Hour)})
	}
	return [][]token.ClosedTokenContext{forwards, controls}
}

// The holder process retains actual blinding and Stock ownership; only the
// existing genuine offline issuer signs the externally provisioned batches.
func routeConsoleDescriptorStock(t *testing.T, f *networkAdmissionFixture, send func(any) localAdmissionReply) {
	t.Helper()
	routeConsolePermissionStock(t, f, send, [3]uint32{8, 4, 0})
	for i, challenges := range descriptorRouteChallenges(f) {
		intent := stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{byte(41 + i)}, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: f.profile.NotAfter}
		batch := send(holderCommand{Operation: "begin", Intent: intent})
		if batch.Outcome != "completed" {
			t.Fatal("compiled holder begin", batch.Outcome)
		}
		issued := issuer.IssueCurrent(t.Context(), f.plan, batch.Request, quota.Bootstrap, func() (admission.AuthorityFacts, time.Time, error) {
			return f.authority.issuer(f.plan.KeyBinding.Signer)
		})
		if issued.Outcome != "issued-offline" {
			t.Fatal("actual offline issuer", issued.Outcome)
		}
		if reply := send(holderCommand{Operation: "complete", Payload: issued.Response}); reply.Outcome != "completed" {
			t.Fatal("compiled Stock completion", reply.Outcome)
		}
	}
}

// Independent accepted byte/signature transcripts; the zero ACK commitment is
// public signed input only, never evidence of an actual ready Publisher.
func routeSignedDescriptor(t *testing.T, f *networkAdmissionFixture, caps uint32, introduction [32]byte) ([]byte, [32]byte) {
	t.Helper()
	return routeSignedDescriptorProfile(t, f, caps, introduction, f.profile.Digest)
}

func routeSignedDescriptorProfile(t *testing.T, f *networkAdmissionFixture, caps uint32, introduction, profile [32]byte) ([]byte, [32]byte) {
	t.Helper()
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x71}, 32))
	instance := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x72}, 32))
	defer clear(authority)
	defer clear(instance)
	public := authority.Public().(ed25519.PublicKey)
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), public...))
	at := time.Now().UTC().Truncate(time.Second)
	end := minRouteDeadline(at.Add(60*time.Second), f.profile.NotAfter)
	credential := []byte{0, 3}
	credential = append(credential, public...)
	credential = append(credential, target[:]...)
	credential = append(credential, instance.Public().(ed25519.PublicKey)...)
	for _, value := range []uint64{7, uint64(at.Unix()), uint64(end.Unix())} {
		credential = binary.BigEndian.AppendUint64(credential, value)
	}
	credential = append(credential, f.profile.NetworkID[:]...)
	credential = binary.BigEndian.AppendUint32(credential, caps)
	credential = append(credential, ed25519.Sign(authority, credential)...)
	publication := append([]byte("ardents-service-publication-v3\x00"), credential...)
	publication = append(publication, make([]byte, 32)...)
	commitment := sha256.Sum256(publication)
	publication = append(publication, ed25519.Sign(instance, commitment[:])...)
	digest := sha256.Sum256(publication)
	recipient := binary.BigEndian.AppendUint64(nil, 9)
	recipient = append(recipient, introduction[:]...)
	for _, key := range [][32]byte{{0x73}, {0x74}} {
		recipient = append(recipient, key[:]...)
	}
	for _, seconds := range []uint64{uint64(at.Unix()), uint64(end.Unix())} {
		recipient = binary.BigEndian.AppendUint64(recipient, seconds)
	}
	transcript := []byte("ardents-private-reachability-v3\x00")
	transcript = append(transcript, f.profile.NetworkID[:]...)
	transcript = append(transcript, profile[:]...)
	transcript = append(transcript, digest[:]...)
	transcript = append(transcript, recipient...)
	body := []byte{0, 3}
	body = append(body, f.profile.NetworkID[:]...)
	body = append(body, target[:]...)
	body = append(body, public...)
	body = append(body, digest[:]...)
	body = append(body, profile[:]...)
	body = append(body, recipient...)
	body = binary.BigEndian.AppendUint16(body, uint16(len(publication)))
	body = append(body, publication...)
	return append(body, ed25519.Sign(instance, transcript)...), target
}

func TestRouteCompiledDescriptorBothCarriers(t *testing.T) {
	_ = compiledCommand(t)
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, sockets, certificates := descriptorRouteFixture(t, profile)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			defer closeClock()
			seed := func() networkAuthorityPlan {
				var key [32]byte
				copy(key[:], f.spec.Authority.Public().(ed25519.PublicKey))
				plan := networkAuthorityPlan{Root: filepath.Join(t.TempDir(), "network"), NetworkID: f.spec.NetworkID, Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: clock}
				config, err := networkStateConfig(&plan)
				if err != nil {
					t.Fatal(err)
				}
				owner, err := state.Open(config)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := owner.Accept(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials); err != nil {
					t.Fatal(err)
				}
				if _, err := owner.AcceptClosedProfile(bundle.Profile); err != nil {
					t.Fatal(err)
				}
				if err := owner.Close(); err != nil {
					t.Fatal(err)
				}
				return plan
			}
			var receivers []*joinCommandProcess
			var budgets []string
			descriptorRoot := t.TempDir()
			var resolutionConfig map[string]any
			for _, id := range [][32]byte{{12}, {13}, {14}, {15}, [32]byte{17}} {
				certificate := certificates[id]
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
				budget := routeProcessBudget(t)
				budgets = append(budgets, budget)
				root := t.TempDir()
				if err := os.Chmod(root, 0700); err != nil {
					t.Fatal(err)
				}
				config := map[string]any{"network": seed(), "node_id": id, "spend_root": root, "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				if id == [32]byte{17} {
					config["descriptor_root"] = descriptorRoot
					resolutionConfig = config
				}
				sockets[id]()
				process := startJoinCommandProcess(t, t.Context(), compiledCommand(t), "route", "receive", "--config", hostingConfig(t, config))
				t.Cleanup(func() {
					if !process.waited {
						_ = process.command.Process.Kill()
						_ = process.wait()
					}
				})
				var ready map[string]string
				if err := process.decode.Decode(&ready); err != nil || ready["operation"] != "route.receive" || ready["phase"] != "listening" {
					t.Fatal("compiled issuer receiver not ready", id, err, process.diagnostic.String())
				}
				receivers = append(receivers, process)
			}
			holderRoot := t.TempDir()
			if err := os.Chmod(holderRoot, 0700); err != nil {
				t.Fatal(err)
			}
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			budgets = append(budgets, plan.HostingRoot)
			send, closeHolder := admissionLocalConsole(t, "holder", map[string]any{"root": holderRoot, "network": seed(), "role": admission.AllocationUser, "route": plan})
			routeConsoleDescriptorStock(t, f, send)
			raw, target := routeSignedDescriptor(t, f, 3, [32]byte{18})
			for _, command := range []holderCommand{{Operation: "prefix-open"}, {Operation: "descriptor-publish", Payload: raw}} {
				if reply := send(command); reply.Outcome != "completed" {
					t.Fatal("compiled Descriptor setup", command.Operation, reply.Outcome)
				}
			}
			assertLookup := func() {
				t.Helper()
				reply := send(holderCommand{Operation: "descriptor-lookup", Target: target})
				if reply.Outcome != "completed" || !bytes.Equal(reply.Proof, raw) {
					t.Fatal("compiled lookup changed exact proof", reply.Outcome)
				}
			}
			assertLookup()
			connectOnly, _ := routeSignedDescriptor(t, f, 2, [32]byte{18})
			broken := bytes.Clone(raw)
			broken[len(broken)-1] ^= 1
			for _, command := range []holderCommand{{Operation: "descriptor-publish", Payload: connectOnly}, {Operation: "descriptor-publish", Payload: broken}, {Operation: "descriptor-lookup", Target: [32]byte{99}}} {
				if reply := send(command); reply.Outcome == "completed" {
					t.Fatal("compiled invalid Descriptor accepted", command.Operation)
				}
			}
			for _, introduction := range [][32]byte{{12}} {
				invalidDuty, _ := routeSignedDescriptor(t, f, 3, introduction)
				if reply := send(holderCommand{Operation: "descriptor-publish", Payload: invalidDuty}); reply.Outcome == "completed" {
					t.Fatal("compiled signed Descriptor bypassed current Introduction duty")
				}
			}
			assertLookup()
			resolution := receivers[len(receivers)-1]
			resolution.interrupt(t)
			exitErr := resolution.wait()
			var resolutionExit *exec.ExitError
			if !errors.As(exitErr, &resolutionExit) || resolutionExit.ExitCode() != 130 {
				t.Fatal("resolution did not join for actual restart", exitErr, resolution.diagnostic.String())
			}
			restarted := startJoinCommandProcess(t, t.Context(), compiledCommand(t), "route", "receive", "--config", hostingConfig(t, resolutionConfig))
			t.Cleanup(func() {
				if !restarted.waited {
					_ = restarted.command.Process.Kill()
					_ = restarted.wait()
				}
			})
			var restartReady map[string]string
			if err := restarted.decode.Decode(&restartReady); err != nil || restartReady["operation"] != "route.receive" || restartReady["phase"] != "listening" {
				t.Fatal("resolution failed to restart retained roots", err, restarted.diagnostic.String())
			}
			receivers[len(receivers)-1] = restarted
			assertLookup() // new genuine admitted terminal, original Target/history
			if reply := send(holderCommand{Operation: "prefix-close"}); reply.Outcome != "completed" {
				t.Fatal("compiled prefix did not join", reply.Outcome)
			}
			closeHolder()
			for _, process := range receivers {
				process.interrupt(t)
			}
			for _, process := range receivers {
				err := process.wait()
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 130 {
					t.Fatal("compiled receiver did not join", err, process.diagnostic.String())
				}
			}
			restored, err := reachability.OpenStore(reachability.StoreConfig{Root: descriptorRoot, Network: f.profile.NetworkID})
			if err != nil {
				t.Fatal("compiled receiver Store did not reopen", err)
			}
			stored, outcome, lookupErr := restored.Lookup(target, f.profile.Digest, time.Now())
			closeErr := restored.Close()
			if lookupErr != nil || closeErr != nil || outcome != reachability.Accepted || !bytes.Equal(stored, raw) {
				t.Fatal("compiled ACK did not retain durable proof", outcome, lookupErr, closeErr)
			}
			for _, root := range budgets {
				budget, err := hosting.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				observed, err := budget.Observe(t.Context())
				closeErr := budget.Close()
				if err != nil || closeErr != nil || observed.ReservedBytes != 0 {
					t.Fatal("compiled issuer retained Hosting after join", err, closeErr, observed.ReservedBytes)
				}
			}
		})
	}
}

func TestRouteDescriptorPostCommitRetirementBothCarriers(t *testing.T) {
	t.Run("receiving-cancellation", func(t *testing.T) { testRouteDescriptorRetirement(t, false, "after-commit") })
	t.Run("signed-successor-State", func(t *testing.T) { testRouteDescriptorRetirement(t, true, "after-commit") })
}

func TestRouteDescriptorPreCommitRetirementBothCarriers(t *testing.T) {
	for _, phase := range []string{"before-admission", "after-reservation", "after-spend"} {
		t.Run(phase, func(t *testing.T) {
			t.Run("receiving-cancellation", func(t *testing.T) { testRouteDescriptorRetirement(t, false, phase) })
			t.Run("signed-successor-State", func(t *testing.T) { testRouteDescriptorRetirement(t, true, phase) })
		})
	}
}

func TestRouteDescriptorOriginalCallerRetirementBothCarriers(t *testing.T) {
	for _, phase := range []string{"after-spend", "after-commit"} {
		t.Run(phase, func(t *testing.T) { testRouteDescriptorRetirement(t, false, phase, true) })
	}
}

func testRouteDescriptorRetirement(t *testing.T, stateLoss bool, phase string, callerLoss ...bool) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			f, reservations, certificates := descriptorRouteFixture(t, carrier)
			operationContext, stopOperation := context.WithCancel(t.Context())
			defer stopOperation()
			loseCaller := len(callerLoss) != 0 && callerLoss[0]
			holder := descriptorRouteStock(t, f)
			storeRoot := t.TempDir()
			store, err := reachability.OpenStore(reachability.StoreConfig{Root: storeRoot, Network: f.profile.NetworkID})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := store.Close(); err != nil {
					t.Error(err)
				}
			})
			var controlSpends atomic.Int32
			entered, resume := make(chan struct{}), make(chan struct{})
			var paused atomic.Bool
			pause := func(point string) {
				if point == phase {
					if paused.CompareAndSwap(false, true) {
						close(entered)
					}
					<-resume
				}
			}
			var released sync.Once
			release := func() { released.Do(func() { close(resume) }) }
			defer release()
			receivingContext, stopReceiving := context.WithCancel(t.Context())
			defer stopReceiving()
			var resolutionBudget *hosting.Budget
			var resolutionAdmission *receiving.Owner
			var resolutionRoot string
			var resolutionBinding receiving.Receiver
			var resolutionExpiry time.Time
			var presented []byte
			var presentedDeadline time.Time
			var servers []*routereceiver.Receiver
			for _, id := range [][32]byte{{12}, {13}, {14}, {15}, {17}} {
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
				spendRoot := t.TempDir()
				owner, err := receiving.Open(spendRoot, binding, func() (receiving.Observation, error) { return f.authority.receiver(binding, member.NotAfter()) })
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := owner.Close(); err != nil {
						t.Error(err)
					}
				})
				budget := networkTestBudget(t)
				config := routereceiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id]}
				if id == [32]byte{17} {
					config.DescriptorStore = store
					resolutionBudget = budget
					resolutionAdmission, resolutionRoot, resolutionBinding, resolutionExpiry = owner, spendRoot, binding, member.NotAfter()
					config.Authority.Current = func() (network.RuntimeView, error) {
						// Observe only the actual committed record. This callback
						// blocks authority observation, never supplies an ACK or
						// substitutes successful Store/Network behavior.
						entries, err := os.ReadDir(filepath.Join(storeRoot, "records"))
						if err != nil {
							return network.RuntimeView{}, err
						}
						if len(entries) == 1 && len(entries[0].Name()) == 64 {
							pause("after-commit")
						}
						return f.current()
					}
				}
				config.Admit = func(ctx context.Context, channel routereceiver.Channel, raw []byte) (receiving.Grant, error) {
					class, err := role.AdmissionClass(channel.Hello.Purpose)
					if err != nil {
						return receiving.Grant{}, err
					}
					if id == [32]byte{17} {
						presented = bytes.Clone(raw)
						presentedDeadline = channel.Hello.Deadline
						pause("before-admission")
					}
					grant, err := owner.Accept(ctx, class, raw, channel.Hello.Deadline, func() (func() error, error) {
						release, err := networkTestReservation(t, budget, channel.Hello.Deadline)
						if err != nil {
							return nil, err
						}
						returned, err := channel.HoldReservation(release)
						if err == nil && id == [32]byte{17} {
							pause("after-reservation")
						}
						return returned, err
					})
					if err == nil && class == admission.ControlClass {
						controlSpends.Add(1)
						pause("after-spend")
					}
					return grant, err
				}
				reservations[id]()
				serverContext := t.Context()
				if id == [32]byte{17} {
					serverContext = receivingContext
				}
				server, err := routereceiver.Listen(serverContext, config)
				if err != nil {
					t.Fatal(err)
				}
				servers = append(servers, server)
				t.Cleanup(func() {
					if err := server.Close(); err != nil {
						if !descriptorRetirementOnly(err, stateLoss, loseCaller) {
							t.Error("receiving join", err)
						}
					}
				})
			}
			root := t.TempDir()
			plan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			prefix, err := startRoutePrefix(t.Context(), plan, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				release()
				if err := prefix.close(); err != nil {
					if !descriptorRetirementOnly(err, stateLoss, loseCaller) {
						t.Error("holder join", err)
					}
				}
			}()

			raw, target := routeSignedDescriptor(t, f, 3, [32]byte{18})
			completed := make(chan error, 1)
			go func() { completed <- prefix.publishDescriptor(operationContext, raw) }()
			select {
			case <-entered:
			case err := <-completed:
				t.Fatal("actual Descriptor boundary not reached", phase, err)
			}
			held, err := resolutionBudget.Observe(t.Context())
			wantSpent := int32(0)
			if phase == "after-spend" || phase == "after-commit" {
				wantSpent = 1
			}
			if err != nil || (held.ReservedBytes != 0) != (phase != "before-admission") || controlSpends.Load() != wantSpent {
				t.Fatal("actual spend/reservation boundary differs", phase, err, held.ReservedBytes, controlSpends.Load())
			}
			if loseCaller {
				stopOperation()
			} else if stateLoss {
				f.successor(t)
			} else {
				stopReceiving()
			}
			stillHeld, err := resolutionBudget.Observe(t.Context())
			if err != nil || stillHeld.ReservedBytes != held.ReservedBytes {
				t.Error("retirement returned Hosting before original observer joined", err, stillHeld.ReservedBytes)
			}
			if t.Context().Err() != nil {
				t.Error("holder caller ended instead of receiving authority")
			}
			release()
			operationErr := <-completed
			if operationErr == nil {
				t.Error("obsolete committed Descriptor returned successful ACK")
			}
			if loseCaller && !errors.Is(operationErr, context.Canceled) {
				t.Error("original caller cancellation was not retained", operationErr)
			}
			terminal := prefix.close()
			if repeated := prefix.close(); terminal == nil && repeated != nil || terminal != nil && !errors.Is(repeated, terminal) {
				t.Error("holder changed retained retirement result", terminal, repeated)
			}
			for _, server := range servers {
				terminal := server.Close()
				if repeated := server.Close(); terminal == nil && repeated != nil || terminal != nil && !errors.Is(repeated, terminal) {
					t.Error("receiver changed retained retirement result", terminal, repeated)
				}
			}
			if !stateLoss {
				if err := resolutionAdmission.Close(); err != nil {
					t.Fatal(err)
				}
				reopened, err := receiving.Open(resolutionRoot, resolutionBinding, func() (receiving.Observation, error) {
					return f.authority.receiver(resolutionBinding, resolutionExpiry)
				})
				if err != nil {
					t.Fatal("spent Admission root did not reopen", err)
				}
				grant, replayErr := reopened.Accept(t.Context(), admission.ControlClass, presented, presentedDeadline, func() (func() error, error) {
					return networkTestReservation(t, resolutionBudget, presentedDeadline)
				})
				if replayErr == nil {
					if err := grant.Release(); err != nil {
						t.Error(err)
					}
					if wantSpent != 0 {
						t.Error("cancellation refunded a spent Control token after reopen")
					}
				} else if wantSpent == 0 {
					t.Error("pre-spend refusal burned actual token", replayErr)
				} else if replayErr.Error() != "closed admission token is unavailable\nclosed token is already spent" {
					t.Error("replay refused for another reason, so no-refund is unproven", replayErr)
				}
				if err := reopened.Close(); err != nil {
					t.Error(err)
				}
			}
			after, err := resolutionBudget.Observe(t.Context())
			if err != nil || after.ReservedBytes != 0 || controlSpends.Load() != wantSpent {
				t.Error("retired operation retained work or changed spend", err, after.ReservedBytes, controlSpends.Load())
			}
			got, outcome, err := store.Lookup(target, f.profile.Digest, time.Now())
			wantOutcome := reachability.Stale
			var wantProof []byte
			if phase == "after-commit" {
				wantOutcome, wantProof = reachability.Accepted, raw
			}
			if err != nil || outcome != wantOutcome || !bytes.Equal(got, wantProof) {
				t.Error("retirement changed actual commit boundary", phase, outcome, err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			restored, err := reachability.OpenStore(reachability.StoreConfig{Root: storeRoot, Network: f.profile.NetworkID})
			if err != nil {
				t.Fatal(err)
			}
			got, outcome, lookupErr := restored.Lookup(target, f.profile.Digest, time.Now())
			closeErr := restored.Close()
			if lookupErr != nil || closeErr != nil || outcome != wantOutcome || !bytes.Equal(got, wantProof) {
				t.Error("reopen changed actual commit boundary", phase, outcome, lookupErr, closeErr)
			}
		})
	}
}

// This test oracle accepts only the existing leaf outcomes of this exact
// retirement scenario. An unrelated sibling failure cannot be hidden by a
// joined authority refusal or context.Canceled. These messages are diagnostic
// outcomes, never inputs that authorize runtime work.
func descriptorRetirementOnly(err error, stateLoss, callerLoss bool) bool {
	if err == nil {
		return false
	}
	if err == context.Canceled {
		return true
	}
	if (stateLoss || callerLoss) && framing.TerminalFailureStage(err) == "peer-retired-write" {
		return true
	}
	if (stateLoss || callerLoss) && transport.IsPeerRetirementCause(err) {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !descriptorRetirementOnly(child, stateLoss, callerLoss) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return descriptorRetirementOnly(wrapped.Unwrap(), stateLoss, callerLoss)
	}
	if !stateLoss {
		return false
	}
	if err == io.EOF {
		return true
	}
	switch err.Error() {
	case "closed profile is unavailable", "route profile changed", "retained Route authority unavailable":
		return true
	default:
		return false
	}
}

func TestDescriptorRetirementOracleRejectsUnrelatedCleanup(t *testing.T) {
	authorityLoss := errors.New("closed profile is unavailable")
	if !descriptorRetirementOnly(errors.Join(authorityLoss, io.EOF, context.Canceled), true, false) {
		t.Fatal("exact authority retirement refused")
	}
	unrelated := errors.New("unrelated durable lease release failed")
	for _, err := range []error{unrelated, errors.Join(authorityLoss, unrelated), errors.Join(context.Canceled, unrelated)} {
		if descriptorRetirementOnly(err, true, false) {
			t.Fatal("unrelated cleanup hidden", err)
		}
	}
	if descriptorRetirementOnly(authorityLoss, false, true) || descriptorRetirementOnly(io.EOF, false, false) {
		t.Fatal("another scenario acquired State-loss exceptions")
	}
}
