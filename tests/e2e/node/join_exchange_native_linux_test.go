//go:build linux && node_process_native

package node

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// The independently pinned canonical commands provide both original Rendezvous
// invocations. Signed State, genuine Stock, Forwarding and Hosting provide the
// actual transport. No fixture supplies an accepting pair or native observation.
func TestNativeJoinDuplexAndReopenTCP(t *testing.T) { nativeJoinWorkload(t, "duplex") }
func TestNativeJoinUndercoverageBeforeSpendTCP(t *testing.T) {
	nativeJoinWorkload(t, "undercoverage")
}

func nativeJoinWorkload(t *testing.T, mode string) {
	binary := processfixture.PinnedCommand(t)
	for _, carrier := range []string{epoch.CarrierClosedTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			processfixture.WaitAdmissionHour(t)
			spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
			listeners, certificates := addForwardingNodes(t, &spec, carrier, 1)
			for id := byte(16); id <= 21; id++ {
				domain, subrole := byte(3), byte(1)
				switch {
				case id >= 20:
					domain, subrole = 2, 4
				case id >= 18:
					subrole = 2
				}
				processfixture.AddReceivingFixtureNode(t, &spec, listeners, certificates, carrier, id, domain, subrole)
			}
			var provisioned map[[32]byte][2]string
			if mode == "certificate" {
				provisioned = ProvisionNodeCertificates(t, &spec, certificates, [32]byte{20}, [32]byte{21})
			}
			bundle, err := networkfixture.BuildClosed(spec)
			if err != nil {
				t.Fatal(err)
			}
			networkinputs.PrepareIssuer(t, &issuerPlan, inventory, spec, bundle.Nodes[0], sha256.Sum256(bundle.Profile))
			networkClock, closeNetworkClock := networkinputs.StartObservedClock(t)
			defer closeNetworkClock()
			clock, closeClock := networkinputs.StartObservedClock(t)
			stopClock := sync.OnceFunc(closeClock)
			defer stopClock()
			seed := func(clock string) state.AuthorityDeclaration {
				return networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)
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
			userStock := fundNativeJoinStock(t, spec, bundle, issuerPlan, authority.CurrentRuntime, admission.AllocationUser)
			publisherStock := fundNativeJoinStock(t, spec, bundle, issuerPlan, authority.CurrentRuntime, admission.AllocationPublisher)
			routeAuthority := funding.Observe(authority.CurrentRuntime, authority.CurrentRuntimeWitness, funding.PresentationGuard(authority.CurrentRuntime))
			var peers []*processfixture.NativeProcess
			for nodeID := byte(12); nodeID <= 19; nodeID++ {
				id := [32]byte{nodeID}
				cert, key := processfixture.WriteNativeCredential(t, certificates[id])
				plan := map[string]any{"network": seed(networkClock), "node_id": id, "process_profile": processresource.Profile,
					"participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": t.TempDir(),
					"hosting_root": processfixture.ProcessBudgetWithCapacity(t, 256), "certificate": cert, "private_key": key,
					"work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
				if err := listeners[id].Close(); err != nil {
					t.Fatal(err)
				}
				peers = append(peers, processfixture.StartFreshRole(t, "forwarding", binary, writeNodePlan(t, plan), nodeID))
			}
			processes := map[[32]byte]*processfixture.NativeProcess{}
			plans := map[[32]byte]map[string]any{}
			before := map[[32]byte][]byte{}
			for _, id := range [][32]byte{{20}, {21}} {
				var cert, key string
				if pair, ok := provisioned[id]; ok {
					cert, key = pair[0], pair[1]
				} else {
					cert, key = processfixture.WriteNativeCredential(t, certificates[id])
				}
				work := uint64(32 << 20)
				if mode == "undercoverage" {
					work--
				}
				plan := map[string]any{"network": seed(clock), "node_id": id, "process_profile": processresource.JoinProfile, "participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": t.TempDir(), "hosting_root": processfixture.ProcessBudgetWithCapacity(t, 256), "certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: work}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
				if err := listeners[id].Close(); err != nil {
					t.Fatal(err)
				}
				processes[id] = processfixture.StartFreshRole(t, "join", binary, writeNodePlan(t, plan), id[0])
				plans[id] = plan
				before[id], err = os.ReadFile(filepath.Join(plan["spend_root"].(string), "closed-token-spends"))
				if err != nil {
					t.Fatal(err)
				}
			}
			makePlan := func(domain uint8) funding.Plan {
				root := t.TempDir()
				return funding.Plan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), SourceInteriorRoot: filepath.Join(root, "source-interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: domain, Deadline: time.Now().Add(60 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			}
			userPlan, publisherPlan := makePlan(1), makePlan(3)
			userContext, err := session.OpenJoinContext(t.Context(), userPlan, routeAuthority, userStock)
			if err != nil {
				t.Fatal(err)
			}
			defer userContext.Close()
			user, err := userContext.Open(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer user.Close()
			publisherContext, err := session.OpenJoinContext(t.Context(), publisherPlan, routeAuthority, publisherStock)
			if err != nil {
				t.Fatal(err)
			}
			defer publisherContext.Close()
			publisher, err := publisherContext.Open(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer publisher.Close()
			recipient, err := user.Recipient(0)
			if err != nil {
				t.Fatal(err)
			}
			intent := session.JoinIntent{Node: recipient.Node, Generation: recipient.Generation, Deadline: minNativeJoinDeadline(userPlan.Deadline, publisherPlan.Deadline, recipient.NotAfter), SetupDeadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}
			if _, err := rand.Read(intent.Secret[:]); err != nil {
				t.Fatal(err)
			}
			if _, err := rand.Read(intent.Context[:]); err != nil {
				t.Fatal(err)
			}
			publisherIntent := intent
			if mode == "context" || mode == "secret" {
				intent.SetupDeadline = time.Now().Add(3 * time.Second).UTC().Truncate(time.Second)
				publisherIntent = intent
				if mode == "context" {
					publisherIntent.Context[0] ^= 1
				} else {
					publisherIntent.Secret[0] ^= 1
				}
			}

			type outcome struct {
				side   int
				stream net.Conn
				err    error
			}
			opened := make(chan outcome, 2)
			go func() { s, e := user.Join(t.Context(), intent); opened <- outcome{0, s, e} }()
			go func() { s, e := publisher.Join(t.Context(), publisherIntent); opened <- outcome{1, s, e} }()
			var streams [2]net.Conn
			var failure error
			for range 2 {
				r := <-opened
				streams[r.side] = r.stream
				failure = errors.Join(failure, r.err)
			}
			defer func() {
				for _, s := range streams {
					if s != nil {
						_ = s.Close()
					}
				}
			}()
			if mode == "undercoverage" {
				if failure == nil {
					t.Fatal("undercovered complete class admitted")
				}
				for id, plan := range plans {
					after, e := os.ReadFile(filepath.Join(plan["spend_root"].(string), "closed-token-spends"))
					if e != nil || !bytes.Equal(before[id], after) {
						t.Fatal("undercoverage spent durable rights", e)
					}
				}
				_ = user.Close()
				_ = publisher.Close()
				_ = userContext.Close()
				_ = publisherContext.Close()
				for _, p := range processes {
					p.Stop(t, true)
				}
				return
			}
			if mode == "context" || mode == "secret" {
				if failure == nil {
					t.Fatal("mismatched original JOIN accepted")
				}
				for id, plan := range plans {
					if id == recipient.Node {
						raw, e := os.ReadFile(filepath.Join(plan["spend_root"].(string), "closed-token-spends"))
						if e != nil || len(raw) <= len(before[id]) {
							t.Fatal("genuine refusal never spent", e)
						}
					}
				}
				_ = user.Close()
				_ = publisher.Close()
				_ = userContext.Close()
				_ = publisherContext.Close()
				for _, p := range processes {
					p.Stop(t, true)
				}
				return
			}

			if failure != nil {
				t.Fatal("actual original paired JOIN", failure)
			}
			// More than one credit window, bounded duplex bytes and directional EOF.
			for side := range 2 {
				payload := bytes.Repeat([]byte{byte(71 + side)}, 96<<10)
				done := make(chan error, 1)
				go func() { _, e := streams[side].Write(payload); done <- e }()
				got := make([]byte, len(payload))
				_, e := io.ReadFull(streams[1-side], got)
				if e = errors.Join(e, <-done); e != nil || !bytes.Equal(got, payload) {
					t.Fatal("native JOIN duplex byte oracle", e)
				}
				if side == 0 && (mode == "caller" || mode == "process-loss" || mode == "clock-loss") {
					p := processes[recipient.Node]
					if mode == "caller" {
						p.Stop(t, true)
					} else {
						if mode == "clock-loss" {
							stopClock()
						} else if e := exec.Command("systemctl", "set-property", "--runtime", p.Unit, "CPUQuota=50%").Run(); e != nil {
							t.Fatal(e)
						}
						e := p.Wait()
						var exit *exec.ExitError
						var event map[string]string
						if !errors.As(e, &exit) || exit.ExitCode() != 1 || json.Unmarshal([]byte(p.Diagnostic.String()), &event) != nil || event["outcome"] != "failed" {
							t.Fatal("original post-I/O loss became success", e, p.Diagnostic.String())
						}
					}
					for _, stream := range streams {
						_ = stream.Close()
					}
					_ = user.Close()
					_ = publisher.Close()
					_ = userContext.Close()
					_ = publisherContext.Close()
					for id, plan := range plans {
						raw, e := os.ReadFile(filepath.Join(plan["spend_root"].(string), "closed-token-spends"))
						if e != nil || id == recipient.Node && len(raw) <= len(before[id]) {
							t.Fatal("post-I/O loss refunded spends", e)
						}
					}
					if mode != "clock-loss" {
						for _, other := range processes {
							if !other.Joined {
								other.Stop(t, true)
							}
						}
					}
					return
				}

				writer, ok := streams[side].(interface{ CloseWrite() error })
				if !ok {
					t.Fatal("joined stream cannot end one direction")
				}
				if e := writer.CloseWrite(); e != nil {
					t.Fatal(e)
				}
				var one [1]byte
				if n, e := streams[1-side].Read(one[:]); n != 0 || e != io.EOF {
					t.Fatal("directional EOF", n, e)
				}
			}
			for _, s := range streams {
				if err := s.Close(); err != nil {
					t.Fatal("typed CLOSE", err)
				}
			}
			if err := errors.Join(wrapNativeJoinClose("user", user.Close()), wrapNativeJoinClose("publisher", publisher.Close()), wrapNativeJoinClose("user-context", userContext.Close()), wrapNativeJoinClose("publisher-context", publisherContext.Close())); err != nil {
				t.Fatal(err)
			}
			for _, peer := range peers {
				peer.Stop(t)
			}
			for id, p := range processes {
				p.Stop(t)
				plan := plans[id]
				after, e := os.ReadFile(filepath.Join(plan["spend_root"].(string), "closed-token-spends"))
				if e != nil {
					t.Fatal(e)
				}
				if id == recipient.Node && len(after) <= len(before[id]) {
					t.Fatal("no independent receiving spend")
				}
				budget, e := hosting.Open(plan["hosting_root"].(string))
				if e != nil {
					t.Fatal(e)
				}
				snapshot, e := budget.Observe(t.Context())
				if closeErr := budget.Close(); e != nil || closeErr != nil || snapshot.ReservedBytes != 0 {
					t.Fatal("unjoined original reservations", e, closeErr, snapshot.ReservedBytes)
				}
				// A separate invocation reopens exact roots and the same ephemeral endpoint.
				fresh := processfixture.StartFreshRole(t, "join", binary, writeNodePlan(t, plan), id[0])
				fresh.Stop(t)
				retained, e := os.ReadFile(filepath.Join(plan["spend_root"].(string), "closed-token-spends"))
				if e != nil || !bytes.Equal(retained, after) {
					t.Fatal("reopen reset spends", e)
				}
			}
		})
	}
}

func TestNativeJoinOriginalCommandLossAfterIOTCP(t *testing.T) {
	for _, mode := range []string{"caller", "process-loss", "clock-loss"} {
		t.Run(mode, func(t *testing.T) { nativeJoinWorkload(t, mode) })
	}
}
func TestNativeJoinMismatchedPairRefusesTCP(t *testing.T) {
	for _, mode := range []string{"context", "secret"} {
		t.Run(mode, func(t *testing.T) { nativeJoinWorkload(t, mode) })
	}
}

func wrapNativeJoinClose(owner string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("original %s close: %w", owner, err)
}

func TestNativeJoinProvisionedCertificateTCP(t *testing.T) { nativeJoinWorkload(t, "certificate") }

func fundNativeJoinStock(t *testing.T, spec networkfixture.ClosedSpec, bundle networkfixture.Closed, plan issuer.Plan, read func() (network.RuntimeView, error), role admission.AllocationRole) *stock.Owner {
	t.Helper()
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		view, err := read()
		if err != nil {
			return admission.AuthorityFacts{}, time.Time{}, err
		}
		return funding.NetworkFacts(view)
	}
	facts, now, err := observe()
	if err != nil {
		t.Fatal(err)
	}
	maxima := [3]uint32{0, 10, 0}
	holder := processfixture.OpenPermittedStock(t, t.TempDir(), role, maxima, observe, facts, spec.Authority, now)
	if status := holder.Status(); !status.Accepted || status.Remaining != maxima {
		t.Fatalf("JOIN allocation maxima changed: %+v", status)
	}
	var challenges []token.ClosedTokenContext
	for _, member := range bundle.Nodes[1:] {
		challenges = append(challenges, token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest,
			IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.Generation,
			Class: uint8(admission.ForwardClass), WindowStart: now.UTC().Truncate(time.Hour)})
	}
	attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{31, byte(role)}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter})
	if err != nil {
		t.Fatal(err)
	}
	batch, _, err := attempt.Request()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := admission.DecodeClosedTokenBatch(batch)
	if err != nil || decoded.Class != uint8(admission.ForwardClass) || len(decoded.BlindedRequests) != 10 {
		t.Fatal("JOIN blinded batch class/count", err)
	}
	issued := issuer.IssueCurrent(t.Context(), plan, batch, quota.Bootstrap, observe)
	if issued.Outcome != "issued-offline" {
		t.Fatal(issued.Outcome)
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

func minNativeJoinDeadline(end time.Time, bounds ...time.Time) time.Time {
	for _, bound := range bounds {
		if bound.Before(end) {
			end = bound
		}
	}
	return end
}
