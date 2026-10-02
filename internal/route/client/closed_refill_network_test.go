//go:build linux

package client

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/node/forwarding"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/node/issuer"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

// Accepted State and offline Permission authority are explicit fixtures. The
// forwarding duties, issuer, tokens/spend roots, Hosting ledger, Carriers and
// nested TLS admission are production owners. A Write gate isolates scheduling;
// it neither fabricates protocol replies nor establishes a network performance
// or installed-worker qualification claim.
type refillWriteGate struct {
	net.Conn
	mu                            sync.Mutex
	armed                         bool
	entered, release, interrupted chan struct{}
	once                          sync.Once
	releaseOnce                   sync.Once
}

func newRefillWriteGate(parent net.Conn) *refillWriteGate {
	return &refillWriteGate{Conn: parent, entered: make(chan struct{}, 1), release: make(chan struct{}), interrupted: make(chan struct{})}
}
func (gate *refillWriteGate) unblock() { gate.releaseOnce.Do(func() { close(gate.release) }) }
func (gate *refillWriteGate) arm()     { gate.mu.Lock(); gate.armed = true; gate.mu.Unlock() }
func (gate *refillWriteGate) Read(value []byte) (int, error) {
	n, err := gate.Conn.Read(value)
	if err != nil {
		gate.once.Do(func() { close(gate.interrupted) })
	}
	return n, err
}
func (gate *refillWriteGate) Write(value []byte) (int, error) {
	gate.mu.Lock()
	armed := gate.armed
	gate.mu.Unlock()
	if armed {
		select {
		case gate.entered <- struct{}{}:
		default:
		}
		select {
		case <-gate.release:
		case <-gate.interrupted:
			return 0, net.ErrClosed
		}
	}
	return gate.Conn.Write(value)
}

func admittedRefillNetwork(t *testing.T, transport carrier.CarrierProfile) (*ClosedSourcePrefix, ClosedTokenPresenter, []byte, [2]*refillWriteGate) {
	t.Helper()
	selected, source := sourceResolutionSelectionFixture(t)
	profile := &source.view.Profile
	now := time.Now().UTC().Truncate(time.Second)
	profile.NotBefore, profile.NotAfter = now.Truncate(time.Hour), now.Truncate(time.Hour).Add(time.Hour)
	source.snapshot.EpochValidFrom, source.snapshot.ValidUntil = profile.NotBefore, profile.NotAfter
	certificates := [3]tls.Certificate{}
	reserved := make([]func(), 3)
	for index := range certificates {
		certificates[index] = entryBindingCertificate(t, int64(51+index))
		candidate := &source.snapshot.Candidates[index]
		candidate.PublicKey = identifierFromKey(certificates[index].PrivateKey.(ed25519.PrivateKey).Public().(ed25519.PublicKey))
		candidate.CarrierProfile = string(transport)
		candidate.ValidFrom, candidate.ValidUntil, candidate.AssignmentNotAfter = profile.NotBefore, profile.NotAfter, profile.NotAfter
		if transport == carrier.ClosedCarrierTCP {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			candidate.Endpoint = listener.Addr().String()
			reserved[index] = func() { _ = listener.Close() }
		} else {
			socket, err := net.ListenPacket("udp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			candidate.Endpoint = socket.LocalAddr().String()
			reserved[index] = func() { _ = socket.Close() }
		}
		t.Cleanup(reserved[index])
	}
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	copy(profile.IssuanceAuthorityKey[:], signer.Public().(ed25519.PublicKey))
	issuerRoot := filepath.Join(t.TempDir(), "issuer")
	receipt, err := credential.InitializeClosedIssuerRoot(credential.ClosedIssuerRootConfig{Root: issuerRoot, NetworkID: profile.NetworkID, NodeID: profile.IssuerNodeID, IdentityKey: certificates[2].PrivateKey.(ed25519.PrivateKey), NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := admission.DecodeClosedIssuerProfile(receipt.Profile, certificates[2].PrivateKey.(ed25519.PrivateKey).Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	profile.TokenKeyCount = uint8(len(inventory.Keys))
	for index, key := range inventory.Keys {
		profile.TokenKeys[index].WindowStart = key.WindowStart
		profile.TokenKeys[index].Class = uint8(key.Class)
		copy(profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	tokenIssuer, err := credential.OpenClosedTokenIssuer(credential.ClosedTokenIssuerConfig{Root: issuerRoot, NetworkID: profile.NetworkID, CurrentProfile: func() (state.ClosedProfileView, bool) { return *profile, true }, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	stock := make(map[[33]byte][][]byte)
	var savedRequest []byte
	for batch := 0; batch < 9; batch++ {
		_, holder, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		permission := admission.Permission{NetworkID: profile.NetworkID, IssuerNodeID: profile.IssuerNodeID, DutyGeneration: profile.IssuerDutyGeneration, PermissionID: [32]byte{byte(100 + batch)}, NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, Maxima: [3]uint32{32, 16, 0}, Signature: [64]byte{1}}
		copy(permission.HolderKey[:], holder.Public().(ed25519.PublicKey))
		raw, err := admission.EncodePermission(permission)
		if err != nil {
			t.Fatal(err)
		}
		transcript := append([]byte("ardents-issuance-permission-v1\x00"), raw[:len(raw)-ed25519.SignatureSize]...)
		copy(permission.Signature[:], ed25519.Sign(signer, transcript))
		var contexts []credential.ClosedTokenContext
		if batch == 0 {
			for index := 0; index < 2; index++ {
				for range 8 {
					contexts = append(contexts, credential.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, ReceiverNodeID: source.view.Nodes[index].NodeID, IssuerNodeID: profile.IssuerNodeID, ReceiverDutyGeneration: source.view.Nodes[index].DutyGeneration, Class: 2, WindowStart: profile.NotBefore})
				}
			}
		} else {
			for range 32 {
				contexts = append(contexts, credential.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, ReceiverNodeID: profile.IssuerNodeID, IssuerNodeID: profile.IssuerNodeID, ReceiverDutyGeneration: profile.IssuerDutyGeneration, Class: 1, WindowStart: profile.NotBefore})
			}
		}
		pending, err := credential.PrepareClosedTokenBatch(credential.ClosedTokenBatchConfig{Profile: *profile, Contexts: contexts, Permission: permission, HolderKey: holder, Now: time.Now().UTC()})
		if err != nil {
			t.Fatal(err)
		}
		request := pending.Request()
		nonce := [32]byte{byte(batch + 1)}
		operation, err := terminal.EncodeIssuanceRequest(nonce, request)
		if err != nil {
			t.Fatal(err)
		}
		result, err := tokenIssuer.IssueTerminalOperation(operation)
		if err != nil {
			t.Fatal(err)
		}
		tokens, err := pending.FinalizeTerminalOperation(nonce, result)
		if err != nil {
			t.Fatal(err)
		}
		for index, token := range tokens {
			var key [33]byte
			copy(key[:32], contexts[index].ReceiverNodeID[:])
			key[32] = contexts[index].Class
			stock[key] = append(stock[key], token)
		}
		if batch == 1 {
			savedRequest = request
		}
	}
	if err := tokenIssuer.Close(); err != nil {
		t.Fatal(err)
	}
	var stockMu sync.Mutex
	present := func(hello ardp.Hello, class uint8) ([]byte, error) {
		stockMu.Lock()
		defer stockMu.Unlock()
		var key [33]byte
		copy(key[:32], hello.RecipientNodeID[:])
		key[32] = class
		tokens := stock[key]
		if len(tokens) == 0 {
			return nil, errors.New("fixture token stock exhausted")
		}
		stock[key] = tokens[1:]
		return tokens[0], nil
	}
	facts := authority.Source{CurrentRoute: source.CurrentClosedRoute, CurrentProfile: func() (state.ClosedProfileView, bool) { return *profile, true }}
	for index := 2; index >= 0; index-- {
		candidate := source.snapshot.Candidates[index]
		snapshot := source.snapshot
		snapshot.RecordPresent = true
		snapshot.NodeID = candidate.NodeID
		snapshot.NodePublicKey = candidate.PublicKey
		snapshot.RecordGeneration = source.view.Nodes[index].DutyGeneration
		snapshot.RecordValidFrom = profile.NotBefore
		snapshot.RecordValidUntil = profile.NotAfter
		snapshot.CarrierProfile = string(transport)
		snapshot.ProbeEndpoint = candidate.Endpoint
		duty := state.ProjectNodeDuty(snapshot)
		hostRoot := filepath.Join(t.TempDir(), "host")
		if err := resource.InitializeHosting(hostRoot, resource.HostingPolicy{Provider: "refill fixture", Start: now.Add(-time.Hour), End: now.Add(time.Hour), Unit: "GiB", Quantity: 4, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1 << 20}); err != nil {
			t.Fatal(err)
		}
		host, err := hosting.Open(hostRoot)
		if err != nil {
			t.Fatal(err)
		}
		reserved[index]()
		current := func() (state.NodeDuty, error) { return duty, nil }
		var stop func()
		var drain func(context.Context) error
		if index == 2 {
			handle, err := issuer.Start(issuer.Config{Profile: issuer.Profile{Root: issuerRoot, AdmissionRoot: t.TempDir(), Certificate: certificates[index], ConnectionLimit: 8, DrainTimeout: 2 * time.Second}, Snapshot: duty, Authority: facts, CurrentDuty: current, VerifyAdmission: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
				return hosting.ControlAdmissionVerifier(facts, time.Now, receiver, host)
			}, Now: time.Now, ListenAddress: candidate.Endpoint})
			if err != nil {
				t.Fatal(err)
			}
			stop, drain = handle.Stop, handle.Drain
			t.Cleanup(func() {
				if err := host.Close(); err != nil {
					t.Error(err)
				}
			})
		} else {
			receiver, ok := facts.Receiver(duty, ardp.PurposeForwarding, time.Now())
			if !ok {
				t.Fatal("forwarding receiver unavailable")
			}
			work, termination := resource.HostingTraffic{Tx: 32 << 20, Rx: 32 << 20}, resource.HostingTraffic{Tx: 64 << 10, Rx: 64 << 10}
			handle, err := forwarding.Start(forwarding.Config{Profile: forwarding.Profile{Root: t.TempDir(), Certificate: certificates[index], ConnectionLimit: 8, DrainTimeout: 2 * time.Second}, Snapshot: duty, Receiver: receiver, ListenAddress: candidate.Endpoint, Authority: facts, CurrentDuty: current, VerifyAdmission: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
				return hosting.AdmissionVerifier(facts, time.Now, receiver, host, work, termination)
			}, Replenish: func(receiver route.ClosedRoleReceiver, spends *replay.Ledger) route.ClosedForwardingReplenisher {
				return hosting.Replenisher(facts, time.Now, receiver, host, spends, work, termination)
			}, LiteralEndpoint: forwarding.ValidCarrierEndpoint, Host: host, Now: time.Now})
			if err != nil {
				t.Fatal(err)
			}
			stop, drain = handle.Stop, handle.Drain
		}
		t.Cleanup(func() {
			stop()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := drain(ctx); err != nil {
				t.Error(err)
			}
		})
	}
	plan, err := prepareClosedBootstrap(source, selected.selection, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	plan.deadline = closedSourcePrefixEnd(plan, source.snapshot, time.Now().UTC())
	raw, err := carrier.OpenClosedRoleCarrier(t.Context(), carrier.ClosedRoleCarrierRequest{CarrierProfile: transport, Endpoint: plan.peers[0].endpoint, ExpectedServer: plan.peers[0].key, Deadline: time.Now().Add(10 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	retirement := &closedRoleRetirement{transport: raw}
	if secured, ok := raw.(*tls.Conn); ok {
		retirement.transport = secured.NetConn()
	}
	gates := [2]*refillWriteGate{newRefillWriteGate(raw), nil}
	prefix := &ClosedSourcePrefix{source: source, selection: selected.selection, plan: plan, connection: gates[0], retirement: retirement, stop: func() bool { return true }, interrupted: make(chan struct{}), done: make(chan struct{})}
	if err := admitClosedSourceObserved(prefix.connection, plan, 0, "entry", present, &prefix.hellos[0]); err != nil {
		t.Fatal(err)
	}
	if err := prefix.openChild(t.Context(), plan.peers[1], plan.deadline, time.Now().Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	gates[1] = newRefillWriteGate(prefix.connection)
	prefix.connection = gates[1]
	if err := admitClosedSourceObserved(prefix.connection, plan, 1, "interior", present, &prefix.hellos[1]); err != nil {
		t.Fatal(err)
	}
	if err := prefix.child.activate(); err != nil {
		t.Fatal(err)
	}
	if err := prefix.connection.SetDeadline(plan.deadline); err != nil {
		t.Fatal(err)
	}
	prefix.channels = newClosedSourceChannelOwner(prefix.connection, plan.deadline, retirement.close)
	prefix.channels.transferred = route.ClosedAdmissionFrameBytes + ardp.HeaderSize + 5
	prefix.channels.framing = prefix.child
	prefix.channels.start()
	go prefix.finishAfterChannels()
	t.Cleanup(func() {
		gates[0].unblock()
		gates[1].unblock()
		if err := prefix.Close(); err != nil {
			t.Error(err)
		}
	})
	return prefix, present, savedRequest, gates
}

func TestAdmittedSourceQueuedRefillCancellation(t *testing.T) {
	for _, transport := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		for _, leg := range []string{"entry", "interior"} {
			t.Run(string(transport)+"/"+leg, func(t *testing.T) {
				prefix, present, request, gates := admittedRefillNetwork(t, transport)
				presented := 0
				refillPresented := make(chan struct{}, 4)
				checkedPresent := func(hello ardp.Hello, class uint8) ([]byte, error) {
					if class == 2 {
						presented++
						refillPresented <- struct{}{}
					}
					return present(hello, class)
				}
				if err := prefix.Replenish(t.Context(), checkedPresent); err != nil {
					t.Fatal(err)
				}
				if presented != 0 {
					t.Fatal("unused admitted prefix spent refill token")
				}
				enough := func() bool {
					prefix.channels.mu.Lock()
					outer := prefix.channels.transferred - prefix.channels.refillBase
					prefix.channels.mu.Unlock()
					prefix.child.mu.Lock()
					inner := prefix.child.transferred - prefix.child.refillBase
					prefix.child.mu.Unlock()
					return outer >= closedRefillThreshold && inner >= closedRefillThreshold
				}
				// Same-process retries of a real, already committed issuance batch carry
				// their full padded requests/results through independently admitted TLS
				// children. Each child burns a fresh class-1 token; no counter is seeded.
				exchanges := 0
				for !enough() && exchanges < 250 {
					// Real duty admission limits verification to 128 per second. Pace
					// independently admitted children so a fast runner does not exhaust
					// that unchanged governor before reaching the traffic threshold.
					time.Sleep(30 * time.Millisecond)
					if _, err := prefix.ExchangeIssuer(t.Context(), present, request); err != nil {
						t.Fatalf("accounted issuance %d: %v", exchanges, err)
					}
					exchanges++
				}
				if !enough() {
					t.Fatal("genuine traffic did not cross both refill thresholds")
				}
				t.Logf("actual admitted %s prefix crossed threshold after %d issuer exchanges", transport, exchanges)
				gate := gates[0]
				if leg == "interior" {
					gate = gates[1]
				}
				gate.arm()
				sibling := make(chan error, 1)
				go func() { _, err := prefix.ExchangeIssuer(t.Context(), present, request); sibling <- err }()
				select {
				case <-gate.entered:
				case <-time.After(2 * time.Second):
					t.Fatal("valid sibling never reached write gate")
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				refill := make(chan error, 1)
				go func() { refill <- prefix.Replenish(ctx, checkedPresent) }()
				if leg == "interior" {
					waitSourceChannelState(t, prefix.channels, func() bool {
						for _, queued := range prefix.channels.controls {
							if queued.frame.Kind == ardp.KindAdmit {
								return true
							}
						}
						return false
					})
				} else {
					select {
					case <-refillPresented:
					case <-time.After(2 * time.Second):
						t.Fatal("inner refill never presented its real token")
					}
				}
				cancel()
				select {
				case err := <-refill:
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("admitted refill: %v", err)
					}
				case <-time.After(time.Second):
					gate.unblock()
					<-refill
					t.Fatal("admitted refill ignored caller cancellation")
				}
				select {
				case err := <-sibling:
					t.Fatalf("canceled refill interrupted sibling: %v", err)
				default:
				}
				prefix.child.mu.Lock()
				innerPending, innerTerminal := prefix.child.refill != nil, prefix.child.terminal
				prefix.child.mu.Unlock()
				prefix.channels.mu.Lock()
				outerPending, outerTerminal := prefix.channels.refill != nil, prefix.channels.terminal
				prefix.channels.mu.Unlock()
				if innerPending || outerPending || innerTerminal != nil || outerTerminal != nil {
					t.Fatalf("unemitted cancellation damaged retained prefix: %t %t %v %v", innerPending, outerPending, innerTerminal, outerTerminal)
				}
				gate.unblock()
				if err := <-sibling; err != nil {
					t.Fatalf("sibling completion: %v", err)
				}
				// A successful real refill now proves there was no late canceled ADMIT
				// or abandoned ACCEPT witness. Both receiving duties remain usable.
				if err := prefix.Replenish(t.Context(), present); err != nil {
					t.Fatalf("real refill after cancellation: %v", err)
				}
				if enough() {
					t.Fatal("real ACCEPT did not advance refill accounting")
				}
			})
		}
	}
}
