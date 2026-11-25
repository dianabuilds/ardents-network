package node_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRoutePairedJoin runs both original holder processes and ten receiving
// processes using signed network authority and actual issued Admission rights.
func TestRoutePairedJoin(t *testing.T) {
	testRouteJoin(t, false)
}

func TestRoutePairedJoinEncryptedTraffic(t *testing.T) {
	testRouteJoin(t, false, "encrypted")
}

func TestRouteRetainsFailedPrefixClose(t *testing.T) {
	testRouteJoin(t, true)
}

func TestRouteJoinRejectsMismatchedIntent(t *testing.T) {
	for _, field := range []string{"secret", "context"} {
		t.Run(field, func(t *testing.T) { testRouteJoin(t, false, field) })
	}
}

func TestRouteSharedPeersSurviveOtherPrefixClose(t *testing.T) {
	testRouteJoin(t, false, "shared-peers")
}

func TestRouteJoinContextRetainsChoicesAcrossReopen(t *testing.T) {
	testRouteJoin(t, false, "reopen")
}

func TestRouteRefillCancellationDuringCurrentness(t *testing.T) {
	for checkpoint := 1; checkpoint <= 4; checkpoint++ {
		t.Run(strconv.Itoa(checkpoint), func(t *testing.T) {
			testRouteJoin(t, false, "cancel-refill-current-"+strconv.Itoa(checkpoint))
		})
	}
}

func TestRouteRefillCancellationAfterReservation(t *testing.T) {
	testRouteJoin(t, false, "cancel-refill-reserve")
}

func TestRouteRefillCancellationAfterReceiverSpend(t *testing.T) {
	testRouteJoin(t, false, "cancel-refill-spend")
}

func TestRouteRefillCancellationDuringInteriorPresentation(t *testing.T) {
	testRouteJoin(t, false, "cancel-refill-interior")
}

func TestRouteRefillCancellationAfterPresentation(t *testing.T) {
	testRouteJoin(t, false, "cancel-refill")
}

func TestRoutePrefixRetiresOnNetworkChange(t *testing.T) {
	for _, mode := range []string{"profile-conflict", "successor"} {
		t.Run(mode, func(t *testing.T) { testRouteJoin(t, false, mode) })
	}
}

func TestRoutePrefixCancellationAfterAdmission(t *testing.T) {
	for checkpoint := 1; checkpoint <= 4; checkpoint++ {
		t.Run(strconv.Itoa(checkpoint), func(t *testing.T) {
			testRouteJoin(t, false, "cancel-open-"+strconv.Itoa(checkpoint))
		})
	}
}

func TestRouteJoinCancellationAfterAdmission(t *testing.T) {
	testRouteJoin(t, false, "cancel-after-spend")
}

func TestRouteRetiredJoinHandleCannotAdoptReplacement(t *testing.T) {
	testRouteJoin(t, false, "retired-handle")
}

func testRouteJoin(t *testing.T, retireEntries bool, mismatch ...string) {
	t.Helper()
	sharedPeers := len(mismatch) != 0 && mismatch[0] == "shared-peers"
	reopen := len(mismatch) != 0 && mismatch[0] == "reopen"
	cancelRefillHold := len(mismatch) != 0 && mismatch[0] == "cancel-refill-reserve"
	cancelRefillSpend := len(mismatch) != 0 && mismatch[0] == "cancel-refill-spend"
	cancelRefillInterior := len(mismatch) != 0 && mismatch[0] == "cancel-refill-interior"
	var cancelRefillCurrent int32
	if len(mismatch) != 0 {
		for i := 1; i <= 4; i++ {
			if mismatch[0] == "cancel-refill-current-"+strconv.Itoa(i) {
				cancelRefillCurrent = int32(i)
			}
		}
	}
	cancelRefill := len(mismatch) != 0 && (mismatch[0] == "cancel-refill" || cancelRefillHold || cancelRefillSpend || cancelRefillInterior || cancelRefillCurrent != 0)
	var openCheckpoint int32
	if len(mismatch) != 0 {
		for i := 1; i <= 4; i++ {
			if mismatch[0] == "cancel-open-"+strconv.Itoa(i) {
				openCheckpoint = int32(i)
			}
		}
	}
	cancelOpen := openCheckpoint != 0
	cancelAfterSpend := len(mismatch) != 0 && mismatch[0] == "cancel-after-spend"
	networkChange := len(mismatch) != 0 && (mismatch[0] == "profile-conflict" || mismatch[0] == "successor")
	retiredHandle := len(mismatch) != 0 && (mismatch[0] == "retired-handle" || cancelAfterSpend || cancelOpen || networkChange || cancelRefill)
	trackPair := sharedPeers || reopen || networkChange || cancelRefill
	binary := buildNodeCommand(t)
	processfixture.WaitAdmissionHour(t)
	spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, processfixture.TempDir(t))
	spec.Nodes = spec.Nodes[:1]
	listeners := make(map[[32]byte]net.Listener)
	certificates := make(map[[32]byte]tls.Certificate)
	for i := byte(12); i <= 21; i++ {
		domain, subrole := byte(1), byte(1)
		switch {
		case i < 16:
			if i >= 14 {
				subrole = 2
			}
		case i < 20:
			domain = 3
			if i >= 18 {
				subrole = 2
			}
		default:
			domain, subrole = 2, 4
		}
		processfixture.AddReceivingFixtureNode(t, &spec, listeners, certificates, epoch.CarrierClosedTCP, i, domain, subrole)
	}
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	networkinputs.PrepareIssuer(t, &issuerPlan, inventory, spec, bundle.Nodes[0], sha256.Sum256(bundle.Profile))
	clock, stopClock := networkinputs.StartObservedClock(t)
	defer stopClock()
	var authorityKey [32]byte
	copy(authorityKey[:], spec.Authority.Public().(ed25519.PublicKey))
	seed := func() state.AuthorityDeclaration {
		t.Helper()
		return networkinputs.SeedNetworkState(t, spec.NetworkID, authorityKey, bundle, clock)
	}
	var peers []*routeBootstrapPeer
	var budgets []string
	var forwardingSpends, forwardingConfigs []string
	var rendezvousSpends = make(map[[32]byte]string)
	for i := byte(12); i <= 21; i++ {
		id := [32]byte{i}
		cert, key := processfixture.WriteNativeCredential(t, certificates[id])
		budget := processfixture.ProcessBudget(t)
		budgets = append(budgets, budget)
		spendRoot := processfixture.TempDir(t)
		plan := map[string]any{"network": seed(), "node_id": id, "spend_root": spendRoot, "hosting_root": budget,
			"certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		if cancelRefill && i < 16 {
			plan["work"] = hosting.Traffic{Tx: 16 << 20, Rx: 16 << 20}
		}
		if err := listeners[id].Close(); err != nil {
			t.Fatal(err)
		}
		config := processfixture.WriteConfig(t, plan)
		peers = append(peers, startRouteBootstrapPeer(t, binary, config))
		if (trackPair || cancelOpen) && i < 16 {
			forwardingSpends = append(forwardingSpends, filepath.Join(spendRoot, "closed-token-spends"))
			forwardingConfigs = append(forwardingConfigs, config)
		}
		if (sharedPeers || cancelAfterSpend) && i >= 20 {
			rendezvousSpends[id] = filepath.Join(spendRoot, "closed-token-spends")
		}
	}
	if len(mismatch) != 0 && mismatch[0] == "encrypted" {
		var prefixes [2]session.Prefix
		var contexts [2]session.JoinContext
		var holders [2]*stock.Owner
		var authorities [2]funding.AdmissionAuthority
		deadline := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
		for side, role := range []admission.AllocationRole{admission.AllocationUser, admission.AllocationPublisher} {
			networkPlan := seed()
			authority, err := funding.OpenAdmissionAuthority("", &networkPlan)
			if err != nil {
				t.Fatal("holder authority", err)
			}
			authorities[side] = authority
			defer authority.Close()
			facts, now, err := authority.ObserveFacts()
			if err != nil {
				t.Fatal("holder facts", err)
			}
			holder, err := stock.Open(processfixture.TempDir(t), role, authority.ObserveFacts)
			if err != nil {
				t.Fatal("holder Stock", err)
			}
			holders[side] = holder
			defer holder.Close()
			request, digest, err := holder.Request([3]uint32{0, 10, 0})
			if err != nil {
				t.Fatal("holder request", err)
			}
			if err := holder.Import(digest, processfixture.FixturePermission(t, request, facts, spec.Authority, now)); err != nil {
				t.Fatal("holder permission", err)
			}
			var challenges []token.ClosedTokenContext
			for i := byte(12); i <= 21; i++ {
				challenges = append(challenges, token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
					ReceiverNodeID: [32]byte{i}, ReceiverDutyGeneration: 9, Class: 2, WindowStart: now.UTC().Truncate(time.Hour)})
			}
			attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{91, byte(side)}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter})
			if err != nil {
				t.Fatal("holder issuance", err)
			}
			batch, _, err := attempt.Request()
			if err != nil {
				t.Fatal("holder batch", err)
			}
			issued := admissionLocalCommand(t, binary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch, "kind": quota.Bootstrap})
			if issued.Outcome != "issued-offline" {
				t.Fatal("issuer", issued.Outcome)
			}
			if err := attempt.Complete(issued.Response, nil); err != nil {
				t.Fatal("issuance result", err)
			}
			root := processfixture.TempDir(t)
			budget := processfixture.ProcessBudget(t)
			budgets = append(budgets, budget)
			domain := uint8(1)
			if side == 1 {
				domain = 3
			}
			plan := funding.Plan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), SourceInteriorRoot: filepath.Join(root, "source-interior"), HostingRoot: budget,
				Domain: domain, Deadline: deadline, Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			contexts[side], err = session.OpenJoinContext(t.Context(), plan, funding.Observe(authority.Current, authority.Witness, authority.Presentation), holder)
			if err != nil {
				t.Fatal("JOIN context", side, err)
			}
			defer contexts[side].Close()
			prefixes[side], err = contexts[side].Open(t.Context())
			if err != nil {
				t.Fatal("JOIN Prefix", side, err)
			}
			defer prefixes[side].Close()
		}
		recipient, err := prefixes[0].Recipient(0)
		if err != nil {
			t.Fatal("Rendezvous", err)
		}
		if recipient.NotAfter.Before(deadline) {
			deadline = recipient.NotAfter
		}
		intent := session.JoinIntent{Node: recipient.Node, Generation: recipient.Generation, Deadline: deadline, SetupDeadline: time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)}
		if _, err := rand.Read(intent.Secret[:]); err != nil {
			t.Fatal(err)
		}
		if _, err := rand.Read(intent.Context[:]); err != nil {
			t.Fatal(err)
		}
		defer clear(intent.Secret[:])
		defer clear(intent.Context[:])
		type result struct {
			side   int
			stream net.Conn
			err    error
		}
		opened := make(chan result, 2)
		for side, prefix := range prefixes {
			go func() { stream, err := prefix.Join(t.Context(), intent); opened <- result{side, stream, err} }()
		}
		var streams [2]net.Conn
		for range 2 {
			r := <-opened
			if r.err != nil {
				t.Fatal("paired JOIN", r.side, r.err)
			}
			streams[r.side] = r.stream
		}
		for side, stream := range streams {
			defer stream.Close()
			binding, ok := stream.(interface {
				CheckSourceBinding([32]byte, [32]byte, time.Time, time.Time) error
				CheckResponderBinding([32]byte, [32]byte, time.Time, time.Time) error
			})
			if !ok {
				t.Fatal("JOIN lacks original binding", side)
			}
			check, opposite := binding.CheckSourceBinding, binding.CheckResponderBinding
			if side == 1 {
				check, opposite = opposite, check
			}
			digest := sha256.Sum256(intent.Secret[:])
			if err := check(digest, intent.Context, intent.Deadline, intent.SetupDeadline); err != nil {
				t.Fatal("original binding", side, err)
			}
			if err := opposite(digest, intent.Context, intent.Deadline, intent.SetupDeadline); err == nil {
				t.Fatal("opposite role accepted JOIN", side)
			}
			changedSecret, changedContext := digest, intent.Context
			changedSecret[0] ^= 1
			changedContext[0] ^= 1
			for _, candidate := range []struct {
				secret, context [32]byte
				end, setup      time.Time
			}{
				{changedSecret, intent.Context, intent.Deadline, intent.SetupDeadline}, {digest, changedContext, intent.Deadline, intent.SetupDeadline},
				{digest, intent.Context, intent.Deadline.Add(-time.Second), intent.SetupDeadline}, {digest, intent.Context, intent.Deadline, intent.SetupDeadline.Add(-time.Second)},
			} {
				if err := check(candidate.secret, candidate.context, candidate.end, candidate.setup); err == nil {
					t.Fatal("foreign JOIN binding", side)
				}
			}
		}
		testRouteJoinedTLS(t, streams)
		for side, stream := range streams {
			if err := stream.Close(); err != nil {
				t.Fatal("stream close", side, err)
			}
		}
		select {
		case <-prefixes[1].Done():
			t.Fatal("Responder retired before Prefix close")
		default:
		}
		for side := range prefixes {
			if err := errors.Join(prefixes[side].Close(), contexts[side].Close(), holders[side].Close(), authorities[side].Close()); err != nil {
				t.Fatal("holder joined result", side, err)
			}
		}
		finishRouteJoinPeers(t, peers, budgets)
		return
	}
	if retiredHandle {
		networkPlan := seed()
		var authority funding.AdmissionAuthority
		var networkOwner *state.State
		if networkChange {
			config, err := networkPlan.Config()
			if err != nil {
				t.Fatal("Network config", err)
			}
			networkOwner, err = state.OpenRetained(config)
			if err != nil {
				t.Fatal("genuine Network owner", err)
			}
			authority = funding.NetworkAdmissionAuthority(networkOwner.CurrentRuntime, networkOwner.Close, networkOwner.CurrentRuntimeWitness)
		} else {
			authority, err = funding.OpenAdmissionAuthority("", &networkPlan)
			if err != nil {
				t.Fatal("actual holder authority", err)
			}
		}
		defer authority.Close()
		facts, now, err := authority.ObserveFacts()
		if err != nil {
			t.Fatal("current holder facts", err)
		}
		holderRoot := processfixture.TempDir(t)
		var refillArmed, refillPresented, refillTriggered atomic.Bool
		var refillObservations, refillPresentations atomic.Int32
		var secondPresentationStart atomic.Int64
		var secondPresented atomic.Bool
		var cancelRefillCaller context.CancelFunc
		var presentationSize int64
		var refillBudget string
		var originalReserve uint64
		var originalSpending [][]byte
		observe := func() (admission.AuthorityFacts, time.Time, error) {
			facts, now, err := authority.ObserveFacts()
			if err == nil && cancelRefill && refillArmed.Load() {
				info, statErr := os.Stat(filepath.Join(holderRoot, "attempts"))
				if statErr != nil {
					return facts, now, statErr
				}
				if info.Size() > presentationSize {
					refillPresented.Store(true)
				}
				if cancelRefillInterior && refillPresentations.Load() >= 2 && info.Size() > secondPresentationStart.Load() {
					secondPresented.Store(true)
				}
			}
			return facts, now, err
		}
		current := func() (network.RuntimeView, error) {
			view, err := authority.Current()
			if err == nil && cancelRefillCurrent != 0 && refillArmed.Load() && refillObservations.Add(1) == cancelRefillCurrent {
				if refillTriggered.CompareAndSwap(false, true) {
					cancelRefillCaller()
				}
			}
			if err == nil && cancelRefillCurrent == 0 && cancelRefill && refillArmed.Load() && refillPresented.Load() {
				reached := true
				if cancelRefillHold {
					reserved, readErr := routeBudgetReserved(refillBudget)
					if readErr != nil {
						return view, readErr
					}
					reached = reserved > originalReserve
				}
				if cancelRefillSpend || cancelRefillInterior {
					reached = false
					for i, path := range forwardingSpends {
						raw, readErr := os.ReadFile(path)
						if readErr != nil {
							return view, readErr
						}
						if !bytes.Equal(raw, originalSpending[i]) {
							reached = true
						}
					}
				}
				if cancelRefillInterior && !secondPresented.Load() {
					reached = false
				}
				if reached && refillTriggered.CompareAndSwap(false, true) {
					cancelRefillCaller()
				}
			}
			return view, err
		}
		var presentationMu sync.Mutex
		var refillPresentation stock.Presentation
		present := func(p stock.Presentation) error {
			if err := authority.Presentation(p); err != nil {
				return err
			}
			if cancelRefill && refillArmed.Load() {
				if cancelRefillInterior {
					next := refillPresentations.Load() + 1
					if next == 2 {
						info, err := os.Stat(filepath.Join(holderRoot, "attempts"))
						if err != nil {
							return err
						}
						secondPresentationStart.Store(info.Size())
					}
					refillPresentations.Store(next)
				}
				presentationMu.Lock()
				refillPresentation = p
				presentationMu.Unlock()
			}
			return nil
		}
		holder, err := stock.Open(holderRoot, admission.AllocationUser, observe)
		if err != nil {
			t.Fatal("actual holder Stock", err)
		}
		defer holder.Close()
		request, digest, err := holder.Request([3]uint32{0, 14, 0})
		if err != nil {
			t.Fatal("holder request", err)
		}
		permission := processfixture.FixturePermission(t, request, facts, spec.Authority, now)
		if err := holder.Import(digest, permission); err != nil {
			t.Fatal("holder permission", err)
		}
		var challenges []token.ClosedTokenContext
		for i := byte(12); i <= 21; i++ {
			challenge := token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
				ReceiverNodeID: [32]byte{i}, ReceiverDutyGeneration: 9, Class: 2, WindowStart: now.UTC().Truncate(time.Hour)}
			challenges = append(challenges, challenge)
			if i < 16 && !cancelOpen {
				challenges = append(challenges, challenge)
			}
		}
		intent := stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{42}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter}
		attempt, err := holder.Begin(intent)
		if err != nil {
			t.Fatal("actual Stock batch", err)
		}
		batch, _, err := attempt.Request()
		if err != nil {
			t.Fatal("actual blinded request", err)
		}
		issued := admissionLocalCommand(t, binary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch, "kind": quota.Bootstrap})
		if issued.Outcome != "issued-offline" {
			t.Fatal("actual issuer", issued.Outcome)
		}
		if err := attempt.Complete(issued.Response, nil); err != nil {
			t.Fatal("actual Stock result", err)
		}
		root := processfixture.TempDir(t)
		budget := processfixture.ProcessBudget(t)
		budgets = append(budgets, budget)
		plan := funding.Plan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: budget,
			Domain: 1, Deadline: time.Now().Add(time.Minute).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		if cancelRefill {
			refillBudget = budget
			plan.Work = hosting.Traffic{Tx: 16 << 20, Rx: 16 << 20}
			prefix, err := session.OpenPrefix(t.Context(), plan, funding.Observe(current, authority.Witness, present), holder)
			if err != nil {
				t.Fatal("real Prefix", err)
			}
			defer prefix.Close()
			var spending [][]byte
			for _, path := range forwardingSpends {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal("original Forwarding spend", err)
				}
				spending = append(spending, raw)
			}
			originalSpending = spending
			info, err := os.Stat(filepath.Join(holderRoot, "attempts"))
			if err != nil {
				t.Fatal("holder presentation history", err)
			}
			presentationSize = info.Size()
			originalReserve, err = routeBudgetReserved(refillBudget)
			if err != nil || originalReserve == 0 {
				t.Fatal("original Prefix reservation", originalReserve, err)
			}
			caller, cancel := context.WithCancel(t.Context())
			defer cancel()
			cancelRefillCaller = cancel
			delayed := &e2eDeferredCaller{Context: caller}
			refillArmed.Store(true)
			refillErr := prefix.Replenish(delayed)
			refillArmed.Store(false)
			if !refillTriggered.Load() || !errors.Is(refillErr, context.Canceled) {
				t.Fatal("refill lost original cancellation", refillPresented.Load(), refillTriggered.Load(), refillErr)
			}
			info, err = os.Stat(filepath.Join(holderRoot, "attempts"))
			if err != nil {
				t.Fatal("holder presentation after cancellation", err)
			}
			presented := info.Size() > presentationSize
			if cancelRefillCurrent == 0 && !presented {
				t.Fatal("cancellation preceded the required durable presentation")
			}
			if cancelRefillCurrent != 0 && presented != (cancelRefillCurrent >= 3) {
				t.Fatal("currentness cancellation occurred at the wrong presentation boundary", cancelRefillCurrent, presented)
			}
			if cancelRefillInterior && (refillPresentations.Load() != 2 || !secondPresented.Load()) {
				t.Fatal("cancellation did not follow the second durable presentation", refillPresentations.Load(), secondPresented.Load())
			}
			if cancelRefillCurrent != 0 || (!cancelRefillHold && !cancelRefillSpend && !cancelRefillInterior) {
				reserved, err := routeBudgetReserved(refillBudget)
				if err != nil || reserved != originalReserve {
					t.Fatal("cancelled refill reserved additional capacity", reserved, originalReserve, err)
				}
			}
			if cancelRefillHold {
				reserved, err := routeBudgetReserved(refillBudget)
				if err != nil || reserved <= originalReserve {
					t.Fatal("additional reserve did not remain held", reserved, originalReserve, err)
				}
			}
			presentationMu.Lock()
			used := refillPresentation
			presentationMu.Unlock()
			if presented {
				if used.RecipientNodeID == [32]byte{} {
					t.Fatal("missing refill presentation")
				}
				raw, takeErr := holder.Take(t.Context(), used, 2)
				clear(raw)
				if stock.TransferFailureStage(takeErr) != "stock" {
					t.Fatal("cancelled refill restored token or failed elsewhere", takeErr)
				}
			}
			changed := 0
			changedIndex := -1
			var changedSpend []byte
			for i, path := range forwardingSpends {
				current, err := os.ReadFile(path)
				if err != nil {
					t.Fatal("receiver spend after cancellation", i, err)
				}
				if !bytes.Equal(spending[i], current) {
					changed++
					changedIndex, changedSpend = i, current
				}
			}
			expected := 0
			if cancelRefillSpend || cancelRefillInterior {
				expected = 1
			}
			if changed != expected {
				t.Fatal("unexpected receiving spends after refill cancellation", changed, expected)
			}
			if cancelRefillInterior && (changedIndex >= 2 || (used.RecipientNodeID != [32]byte{14} && used.RecipientNodeID != [32]byte{15})) {
				t.Fatal("refill did not spend at Entry before Interior presentation", changedIndex, used.RecipientNodeID)
			}
			terminal := prefix.Close()
			if cancelRefillSpend {
				if terminal == nil || !refillCancellationTerminal(terminal) || prefix.Close() != terminal {
					t.Fatal("refill lost its retained failed terminal", terminal)
				}
			} else if terminal != nil {
				t.Fatal("joined Prefix", terminal)
			}
			if err := errors.Join(holder.Close(), authority.Close()); err != nil {
				t.Fatal("joined holder", err)
			}
			finishRouteJoinPeers(t, peers, budgets)
			if cancelRefillSpend || cancelRefillInterior {
				reopened := startRouteBootstrapPeer(t, binary, forwardingConfigs[changedIndex])
				reopenedSpend, err := os.ReadFile(forwardingSpends[changedIndex])
				if err != nil || !bytes.Equal(changedSpend, reopenedSpend) {
					t.Fatal("reopened receiver changed the spent refill", err)
				}
				finishRouteJoinPeers(t, []*routeBootstrapPeer{reopened}, nil)
			}
			return
		}
		if networkChange {
			var before [][]byte
			for _, path := range forwardingSpends {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal("original Forwarding journal", err)
				}
				before = append(before, raw)
			}
			prefix, err := session.OpenPrefix(t.Context(), plan, funding.Observe(authority.Current, authority.Witness, authority.Presentation), holder)
			if err != nil {
				t.Fatal("real Prefix", err)
			}
			defer prefix.Close()
			var spent [][]byte
			changed := 0
			for i, path := range forwardingSpends {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal("Forwarding spend", err)
				}
				spent = append(spent, raw)
				if !bytes.Equal(before[i], raw) {
					changed++
				}
			}
			if changed != 2 {
				t.Fatal("Prefix did not spend two real rights", changed)
			}
			nextSpec := spec
			if mismatch[0] == "profile-conflict" {
				nextSpec.IssuanceAuthority[0] ^= 1
			} else {
				nextSpec.Number++
				nextSpec.Previous = bundle.Epoch.Digest
				nextSpec.Seed[0]++
			}
			next, err := networkfixture.BuildClosed(nextSpec)
			if err != nil {
				t.Fatal("signed Network change", err)
			}
			if mismatch[0] == "profile-conflict" {
				if next.Epoch.Digest != bundle.Epoch.Digest {
					t.Fatal("profile conflict changed Epoch")
				}
				if _, err := networkOwner.AcceptClosedProfile(next.Profile); err == nil {
					t.Fatal("conflicting profile accepted")
				}
			} else {
				if _, err := networkOwner.AcceptEpoch(t.Context(), next.Epoch.Raw, next.Epoch.Inputs, next.Epoch.Materials); err != nil {
					t.Fatal("successor Epoch", err)
				}
				if _, err := networkOwner.AcceptClosedProfile(next.Profile); err != nil {
					t.Fatal("successor profile", err)
				}
			}
			select {
			case <-prefix.Done():
			case <-time.After(6 * time.Second):
				t.Fatal("Prefix survived Network change")
			}
			terminal := prefix.Close()
			if terminal == nil || prefix.Close() != terminal {
				t.Fatal("Prefix lost retained refusal", terminal)
			}
			// All receiving processes remain reachable with their original signed
			// state. A replacement Prefix must still refuse under the changed local
			// authority without spending more rights or silently using that old state.
			for i := 1; i <= 4; i++ {
				probe, err := net.DialTimeout("tcp", spec.Nodes[i].Endpoint, time.Second)
				if err != nil {
					t.Fatal("original receiver is unreachable", i, err)
				}
				if err := probe.Close(); err != nil {
					t.Fatal("close receiver probe", err)
				}
			}
			replacement, openErr := session.OpenPrefix(t.Context(), plan, funding.Observe(authority.Current, authority.Witness, authority.Presentation), holder)
			if replacement.Retained() {
				_ = replacement.Close()
				t.Fatal("Network change admitted a replacement Prefix")
			}
			if openErr == nil {
				t.Fatal("Network change lost its replacement refusal")
			}
			for i, path := range forwardingSpends {
				current, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(spent[i], current) {
					t.Fatal("Network retirement or replacement changed spending", i, err)
				}
			}
			if err := errors.Join(holder.Close(), authority.Close()); err != nil {
				t.Fatal("holder joined result", err)
			}
			finishRouteJoinPeers(t, peers, budgets)
			return
		}
		if cancelOpen {
			var before [][]byte
			for _, path := range forwardingSpends {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal("original Forwarding journal", err)
				}
				before = append(before, raw)
			}
			caller, cancel := context.WithCancel(t.Context())
			defer cancel()
			var triggered atomic.Bool
			var observations atomic.Int32
			var presentationMu sync.Mutex
			var presentations []stock.Presentation
			present := func(p stock.Presentation) error {
				if err := authority.Presentation(p); err != nil {
					return err
				}
				presentationMu.Lock()
				presentations = append(presentations, p)
				presentationMu.Unlock()
				return nil
			}
			current := func() (network.RuntimeView, error) {
				view, err := authority.Current()
				if err != nil {
					return view, err
				}
				changed := 0
				for i, path := range forwardingSpends {
					raw, err := os.ReadFile(path)
					if err != nil {
						return view, err
					}
					if !bytes.Equal(before[i], raw) {
						changed++
					}
				}
				if changed == 2 && observations.Add(1) == openCheckpoint && triggered.CompareAndSwap(false, true) {
					cancel()
				}
				return view, nil
			}
			prefix, err := session.OpenPrefix(&e2eDeferredCaller{Context: caller}, plan, funding.Observe(current, authority.Witness, present), holder)
			if prefix.Retained() {
				_ = prefix.Close()
				t.Fatal("cancelled caller received a live Prefix")
			}
			if !triggered.Load() || !errors.Is(err, context.Canceled) {
				t.Fatal("post-spend cancellation was not retained", triggered.Load(), err)
			}
			changed := 0
			for i, path := range forwardingSpends {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal("Forwarding spending after cancellation", err)
				}
				if !bytes.Equal(before[i], raw) {
					changed++
				}
			}
			if changed != 2 {
				t.Fatal("cancelled opening did not retain both real spends", changed)
			}
			presentationMu.Lock()
			burnt := append([]stock.Presentation(nil), presentations...)
			presentationMu.Unlock()
			if len(burnt) != 2 {
				t.Fatal("cancelled opening lacked two actual presentations", len(burnt))
			}
			for _, presentation := range burnt {
				raw, takeErr := holder.Take(t.Context(), presentation, 2)
				clear(raw)
				if stock.TransferFailureStage(takeErr) != "stock" {
					t.Fatal("cancelled opening restored its presented token", takeErr)
				}
			}
			if err := errors.Join(holder.Close(), authority.Close()); err != nil {
				t.Fatal("holder joined result", err)
			}
			finishRouteJoinPeers(t, peers, budgets)
			return
		}
		joinContext, err := session.OpenJoinContext(t.Context(), plan, funding.Observe(authority.Current, authority.Witness, authority.Presentation), holder)
		if err != nil {
			t.Fatal("actual JOIN context", err)
		}
		defer joinContext.Close()
		first, err := joinContext.Open(t.Context())
		if err != nil {
			t.Fatal("first actual Prefix", err)
		}
		choice, err := first.Recipient(0)
		if err != nil {
			t.Fatal("original Rendezvous", err)
		}
		if err := first.Close(); err != nil {
			t.Fatal("original Prefix join", err)
		}
		if other, err := session.OpenJoinContext(t.Context(), plan, funding.Observe(authority.Current, authority.Witness, authority.Presentation), holder); err == nil {
			_ = other.Close()
			t.Fatal("physical Prefix retirement released the original selection lease")
		}
		second, err := joinContext.Open(t.Context())
		if err != nil {
			t.Fatal("replacement actual Prefix", err)
		}
		if _, err := first.Recipient(0); err == nil {
			t.Fatal("retired handle acquired replacement selection")
		}
		join := session.JoinIntent{Node: choice.Node, Generation: choice.Generation, Secret: [32]byte{1}, Context: [32]byte{2},
			Deadline: choice.NotAfter, SetupDeadline: time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)}
		if conn, err := first.Join(t.Context(), join); err == nil || conn != nil {
			t.Fatal("retired handle acquired replacement JOIN", err)
		}
		if err := first.Close(); err != nil {
			t.Fatal("original close result changed", err)
		}
		select {
		case <-second.Done():
			t.Fatal("old handle retirement closed the replacement")
		default:
		}
		if again, err := second.Recipient(0); err != nil || again != choice {
			t.Fatal("old handle retirement changed replacement", err)
		}
		var spendJournal string
		var committedSpend []byte
		if cancelAfterSpend {
			spendJournal = rendezvousSpends[choice.Node]
			before, err := os.ReadFile(spendJournal)
			if err != nil {
				t.Fatal("Rendezvous journal", err)
			}
			caller, cancel := context.WithCancel(t.Context())
			defer cancel()
			type joinResult struct {
				stream net.Conn
				err    error
			}
			result := make(chan joinResult, 1)
			go func() { stream, err := second.Join(caller, join); result <- joinResult{stream, err} }()
			timer := time.NewTimer(3 * time.Second)
			defer timer.Stop()
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				observed, err := os.ReadFile(spendJournal)
				if err != nil {
					t.Fatal("Rendezvous spending", err)
				}
				committed, err := committedRouteSpend(before, observed)
				if err != nil {
					t.Fatal("observe JOIN Admission", err)
				}
				if committed {
					committedSpend = observed
					break
				}
				select {
				case r := <-result:
					t.Fatal("JOIN ended before real Admission", r.err)
				case <-timer.C:
					t.Fatal("JOIN did not reach real Admission")
				case <-ticker.C:
				case <-t.Context().Done():
					t.Fatal("waiting for Admission", t.Context().Err())
				}
			}
			cancel()
			select {
			case r := <-result:
				if r.stream != nil {
					_ = r.stream.Close()
					t.Fatal("cancelled unpaired JOIN returned a stream")
				}
				if !errors.Is(r.err, context.Canceled) {
					t.Fatal("JOIN lost caller cancellation", r.err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("cancelled JOIN did not physically join")
			}
			if _, err := second.Recipient(0); err != nil {
				t.Fatal("cancelled JOIN retired its Prefix", err)
			}
		}
		if err := errors.Join(second.Close(), joinContext.Close(), holder.Close(), authority.Close()); err != nil {
			t.Fatal("JOIN owners did not join", err)
		}
		finishRouteJoinPeers(t, peers, budgets)
		if cancelAfterSpend {
			after, err := os.ReadFile(spendJournal)
			if err != nil || !bytes.Equal(committedSpend, after) {
				t.Fatal("joined receiver did not retain the exact original JOIN spend", err)
			}
		}
		return
	}
	type holder struct {
		send     func(any) localAdmissionReply
		close    func() error
		deadline time.Time
	}
	roles := []admission.AllocationRole{admission.AllocationUser, admission.AllocationPublisher}
	if retireEntries || reopen {
		roles = roles[:1]
	}
	if sharedPeers {
		roles = []admission.AllocationRole{admission.AllocationUser, admission.AllocationUser, admission.AllocationUser, admission.AllocationUser, admission.AllocationUser, admission.AllocationUser, admission.AllocationUser}
	}
	var holders []holder
	var pairs [][2]int
	for side, role := range roles {
		networkPlan := seed()
		config, err := networkPlan.Config()
		if err != nil {
			t.Fatal(err)
		}
		authority, err := state.OpenRetained(config)
		if err != nil {
			t.Fatal(err)
		}
		view, viewErr := authority.CurrentRuntime()
		closeErr := authority.Close()
		if viewErr != nil || closeErr != nil {
			t.Fatal("signed holder authority", viewErr, closeErr)
		}
		facts, now, err := funding.NetworkFacts(view)
		if err != nil {
			t.Fatal(err)
		}
		budget := processfixture.ProcessBudget(t)
		budgets = append(budgets, budget)
		root := processfixture.TempDir(t)
		deadline := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
		domain := uint8(1)
		if side == 1 && !sharedPeers {
			domain = 3
		}
		route := map[string]any{"entry_root": filepath.Join(root, "entry"), "interior_root": filepath.Join(root, "interior"),
			"hosting_root": budget, "domain": domain, "deadline": deadline,
			"work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		if side == 1 && !sharedPeers {
			route["source_interior_root"] = filepath.Join(root, "source-interior")
		}
		send, closeHolder := admissionLocalConsoleResult(t, binary, "holder", map[string]any{"root": processfixture.TempDir(t), "network": networkPlan, "role": role, "route": route})
		holders = append(holders, holder{send: send, close: closeHolder, deadline: deadline})
		count := uint32(10)
		if reopen {
			count = 14
		}
		request := send(holderCommand{Operation: "request", Maxima: [3]uint32{0, count, 0}})
		if request.Outcome != "completed" {
			t.Fatal("holder request", side, request.Outcome)
		}
		permission := processfixture.FixturePermission(t, request.Request, facts, spec.Authority, now)
		if reply := send(holderCommand{Operation: "import", Digest: request.Digest, Payload: permission}); reply.Outcome != "completed" {
			t.Fatal("holder import", side, reply.Outcome)
		}
		var challenges []token.ClosedTokenContext
		for i := byte(12); i <= 21; i++ {
			member, err := view.Member([32]byte{i}, view.ObservedAt())
			if err != nil {
				t.Fatal(err)
			}
			challenges = append(challenges, token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
				ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration, Class: 2, WindowStart: time.Now().UTC().Truncate(time.Hour)})
			if reopen && i < 16 {
				challenges = append(challenges, challenges[len(challenges)-1])
			}
		}
		intent := stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{41, byte(role)}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter}
		batch := send(holderCommand{Operation: "begin", Intent: intent})
		if batch.Outcome != "completed" {
			t.Fatal("holder begin", side, batch.Outcome)
		}
		decoded, err := admission.DecodeClosedTokenBatch(batch.Request)
		if err != nil || decoded.Class != 2 || len(decoded.BlindedRequests) != int(count) {
			t.Fatal("holder batch", err)
		}
		issued := admissionLocalCommand(t, binary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch.Request, "kind": quota.Bootstrap})
		if issued.Outcome != "issued-offline" {
			t.Fatal("issuer response", issued.Outcome)
		}
		result, err := admission.DecodeClosedTokenBatchResult(issued.Response)
		if err != nil || result.Status != admission.ClosedTokenIssued || len(result.Signatures) != int(count) {
			t.Fatal("issuer batch response", err)
		}
		if reply := send(holderCommand{Operation: "complete", Payload: issued.Response}); reply.Outcome != "completed" {
			t.Fatal("holder complete", side, reply.Outcome)
		}
		if sharedPeers && side >= 5 {
			continue
		}
		var before [][]byte
		for _, path := range forwardingSpends {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal("read Forward spending", err)
			}
			before = append(before, raw)
		}
		if reply := send(map[string]any{"operation": "join-prefix-open"}); reply.Outcome != "completed" {
			t.Fatal("JOIN prefix", side, reply.Outcome)
		}
		if trackPair {
			pair := [2]int{-1, -1}
			for i, path := range forwardingSpends {
				after, err := os.ReadFile(path)
				if err != nil {
					t.Fatal("read opened Forward spending", err)
				}
				if !bytes.Equal(before[i], after) {
					if pair[i/2] != -1 {
						t.Fatal("Prefix spent more than one right per forwarding role")
					}
					pair[i/2] = i
				}
			}
			if pair[0] < 0 || pair[1] < 0 {
				t.Fatal("Prefix did not spend both forwarding rights", pair)
			}
			pairs = append(pairs, pair)
		}
	}
	if reopen {
		holder := holders[0]
		var choices [2]localAdmissionReply
		for slot := range choices {
			choices[slot] = holder.send(map[string]any{"operation": "rendezvous", "choice": slot})
			if choices[slot].Outcome != "completed" {
				t.Fatal("initial Rendezvous selection", slot, choices[slot].Outcome)
			}
		}
		if choices[0].Recipient.Node == choices[1].Recipient.Node {
			t.Fatal("Rendezvous alternatives are not distinct")
		}
		if reply := holder.send(map[string]any{"operation": "prefix-close"}); reply.Outcome != "completed" {
			t.Fatal("original JOIN Prefix close", reply.Outcome)
		}
		var before [][]byte
		for _, path := range forwardingSpends {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal("retained forwarding journal", err)
			}
			before = append(before, raw)
		}
		if reply := holder.send(map[string]any{"operation": "join-prefix-open"}); reply.Outcome != "completed" {
			t.Fatal("reopen retained JOIN context", reply.Outcome)
		}
		for i, path := range forwardingSpends {
			current, err := os.ReadFile(path)
			if err != nil || (!bytes.Equal(current, before[i])) != (i == pairs[0][0] || i == pairs[0][1]) {
				t.Fatal("reopen changed retained Entry or Interior", i, err)
			}
		}
		for slot, original := range choices {
			again := holder.send(map[string]any{"operation": "rendezvous", "choice": slot})
			if again.Outcome != "completed" || again.Recipient != original.Recipient {
				t.Fatal("reopen changed Rendezvous selection", slot, again.Outcome)
			}
		}
		if reply := holder.send(map[string]any{"operation": "prefix-close"}); reply.Outcome != "completed" {
			t.Fatal("reopened JOIN Prefix close", reply.Outcome)
		}
		if err := holder.close(); err != nil {
			t.Fatal("JOIN context terminal", err)
		}
	} else if sharedPeers {
		start := make(chan struct{})
		opened := make(chan localAdmissionReply, 2)
		for _, holder := range holders[5:] {
			go func() {
				<-start
				var reply localAdmissionReply
				defer func() { opened <- reply }()
				reply = holder.send(map[string]any{"operation": "join-prefix-open"})
			}()
		}
		close(start)
		for range holders[5:] {
			if reply := <-opened; reply.Outcome != "completed" {
				t.Fatal("concurrent Prefix opening", reply.Outcome)
			}
		}
		first, second := -1, -1
		for i := range pairs {
			for j := 0; j < i; j++ {
				if pairs[i] == pairs[j] {
					first, second = j, i
				}
			}
		}
		if first < 0 {
			t.Fatal("five Prefixes did not share one of four possible peer pairs")
		}
		if reply := holders[first].send(map[string]any{"operation": "prefix-close"}); reply.Outcome != "completed" {
			t.Fatal("first shared Prefix close", reply.Outcome)
		}
		recipient := holders[second].send(map[string]any{"operation": "rendezvous", "choice": 0})
		if recipient.Outcome != "completed" {
			t.Fatal("closing one Prefix retired its shared-peer sibling", recipient.Outcome)
		}
		spendPath, ok := rendezvousSpends[recipient.Recipient.Node]
		if !ok {
			t.Fatal("unknown Rendezvous selected")
		}
		before, err := os.ReadFile(spendPath)
		if err != nil {
			t.Fatal("Rendezvous spending journal", err)
		}
		var secret, joinContext [32]byte
		if _, err := rand.Read(secret[:]); err != nil {
			t.Fatal(err)
		}
		if _, err := rand.Read(joinContext[:]); err != nil {
			t.Fatal(err)
		}
		defer clear(secret[:])
		defer clear(joinContext[:])
		intent := map[string]any{"node": recipient.Recipient.Node, "generation": recipient.Recipient.Generation,
			"secret": secret, "context": joinContext, "deadline": holders[second].deadline,
			"setup_deadline": time.Now().Add(3 * time.Second).UTC().Truncate(time.Second)}
		if reply := holders[second].send(map[string]any{"operation": "join-open", "join": intent}); reply.Outcome != "refused" {
			t.Fatal("unpaired JOIN unexpectedly completed", reply.Outcome)
		}
		after, err := os.ReadFile(spendPath)
		if err != nil || bytes.Equal(before, after) {
			t.Fatal("remaining Prefix did not reach the actual Rendezvous", err)
		}
		for i, holder := range holders {
			if i != first {
				if reply := holder.send(map[string]any{"operation": "prefix-close"}); reply.Outcome != "completed" {
					t.Fatal("shared Prefix close", i, reply.Outcome)
				}
			}
			if err := holder.close(); err != nil {
				t.Fatal("shared Prefix holder terminal", i, err)
			}
		}
	} else {
		chosen := holders[0].send(map[string]any{"operation": "rendezvous", "choice": 0})
		if chosen.Outcome != "completed" || (chosen.Recipient.Node != [32]byte{20} && chosen.Recipient.Node != [32]byte{21}) || chosen.Recipient.Generation != 9 || chosen.Recipient.NotAfter.IsZero() {
			t.Fatal("Rendezvous selection", chosen.Outcome)
		}
		deadline := chosen.Recipient.NotAfter
		for _, holder := range holders {
			if holder.deadline.Before(deadline) {
				deadline = holder.deadline
			}
		}
		var secret, joinContext [32]byte
		if _, err := rand.Read(secret[:]); err != nil {
			t.Fatal(err)
		}
		if _, err := rand.Read(joinContext[:]); err != nil {
			t.Fatal(err)
		}
		defer clear(secret[:])
		defer clear(joinContext[:])
		intent := map[string]any{"node": chosen.Recipient.Node, "generation": chosen.Recipient.Generation, "secret": secret, "context": joinContext,
			"deadline": deadline, "setup_deadline": time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)}
		if retireEntries {
			for _, peer := range peers[:2] {
				if err := processfixture.Interrupt(peer.command); err != nil {
					t.Fatal(err)
				}
			}
			for _, peer := range peers[:2] {
				err := peer.command.Wait()
				peer.waited = true
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 130 {
					t.Fatal("Entry did not join", err)
				}
			}
			if reply := holders[0].send(map[string]any{"operation": "join-open", "join": intent}); reply.Outcome != "refused" {
				t.Fatal("JOIN through retired Entry", reply.Outcome)
			}
			for range 2 {
				if reply := holders[0].send(map[string]any{"operation": "prefix-close"}); reply.Outcome != "refused" {
					t.Fatal("failed prefix close changed", reply.Outcome)
				}
			}
			if reply := holders[0].send(map[string]any{"operation": "join-prefix-open"}); reply.Outcome != "refused" {
				t.Fatal("failed prefix was replaced", reply.Outcome)
			}
			var exit *exec.ExitError
			if err := holders[0].close(); !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatal("EOF erased failed prefix close", err)
			}
		} else {
			results := make(chan localAdmissionReply, 2)
			for side, holder := range holders {
				sideIntent := make(map[string]any, len(intent))
				for key, value := range intent {
					sideIntent[key] = value
				}
				if side == 1 && len(mismatch) != 0 {
					switch mismatch[0] {
					case "secret":
						changed := secret
						changed[0] ^= 1
						sideIntent["secret"] = changed
					case "context":
						changed := joinContext
						changed[0] ^= 1
						sideIntent["context"] = changed
					default:
						t.Fatal("unknown JOIN mismatch", mismatch[0])
					}
				}
				go func() {
					var reply localAdmissionReply
					defer func() { results <- reply }()
					reply = holder.send(map[string]any{"operation": "join-open", "join": sideIntent})
				}()
			}
			for range holders {
				reply := <-results
				if len(mismatch) == 0 && reply.Outcome != "completed" {
					t.Fatal("paired JOIN", reply.Outcome)
				}
				if len(mismatch) != 0 && reply.Outcome != "refused" {
					t.Fatal("mismatched JOIN accepted", mismatch[0], reply.Outcome)
				}
			}
			for side, holder := range holders {
				operations := []string{"join-close", "prefix-close"}
				if len(mismatch) != 0 {
					operations = []string{"prefix-close"}
				}
				for _, operation := range operations {
					if reply := holder.send(map[string]any{"operation": operation}); reply.Outcome != "completed" {
						t.Fatal("holder join", side, operation, reply.Outcome)
					}
				}
				if err := holder.close(); err != nil {
					t.Fatal("holder terminal", side, err)
				}
			}
		}
	}
	finishRouteJoinPeers(t, peers, budgets)
}

// A cancelled post-spend refill can fail its physical acknowledgement. Every
// retained leaf must still be that failure or the original cancellation.
func refillCancellationTerminal(err error) bool {
	if err == nil {
		return false
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		children := many.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !refillCancellationTerminal(child) {
				return false
			}
		}
		return true
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return refillCancellationTerminal(one.Unwrap())
	}
	return err == context.Canceled || err.Error() == "route refill acknowledgement refused"
}

func finishRouteJoinPeers(t *testing.T, peers []*routeBootstrapPeer, budgets []string) {
	t.Helper()
	for _, peer := range peers {
		if peer.waited {
			continue
		}
		if err := processfixture.Interrupt(peer.command); err != nil {
			t.Fatal(err)
		}
	}
	for _, peer := range peers {
		if peer.waited {
			continue
		}
		err := peer.command.Wait()
		peer.waited = true
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 130 {
			t.Fatal("receiver did not join", err, peer.diagnostic.String())
		}
	}
	for _, root := range budgets {
		owner, err := hosting.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		observation, observeErr := owner.Observe(t.Context())
		closeErr := owner.Close()
		if observeErr != nil || closeErr != nil || observation.ReservedBytes != 0 {
			t.Fatal("JOIN retained Hosting", observeErr, closeErr, observation.ReservedBytes)
		}
	}
}

func testRouteJoinedTLS(t *testing.T, streams [2]net.Conn) {
	t.Helper()
	var certs [2]tls.Certificate
	var keys [2]ed25519.PublicKey
	for i := range certs {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { clear(private) })
		keys[i], certs[i] = public, processfixture.RoleCertificate(t, private)
	}
	pin := func(expected ed25519.PublicKey) func(tls.ConnectionState) error {
		return func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 1 {
				return errors.New("fixture peer absent")
			}
			key, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
			if !ok || !bytes.Equal(key, expected) {
				return errors.New("fixture peer changed")
			}
			return nil
		}
	}
	client := tls.Client(streams[0], &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certs[0]}, InsecureSkipVerify: true, VerifyConnection: pin(keys[1])})
	server := tls.Server(streams[1], &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certs[1]}, ClientAuth: tls.RequireAnyClientCert, VerifyConnection: pin(keys[0])})
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	if err := errors.Join(client.SetDeadline(time.Now().Add(15*time.Second)), server.SetDeadline(time.Now().Add(15*time.Second))); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.HandshakeContext(ctx) }()
	if err := errors.Join(client.HandshakeContext(ctx), <-done); err != nil {
		t.Fatal("opaque TLS through joined Route", err)
	}
	payload := bytes.Repeat([]byte{0x42}, 96<<10)
	go func() { _, err := client.Write(payload); done <- err }()
	got := make([]byte, len(payload))
	_, readErr := io.ReadFull(server, got)
	if err := errors.Join(readErr, <-done); err != nil || !bytes.Equal(got, payload) {
		t.Fatal("credit-controlled encrypted payload", err)
	}
	go func() { done <- server.CloseWrite() }()
	if err := errors.Join(client.CloseWrite(), <-done); err != nil {
		t.Fatal("fixture TLS terminal", err)
	}
	var one [1]byte
	if _, err := client.Read(one[:]); err != io.EOF {
		t.Fatal("fixture client terminal", err)
	}
	if _, err := server.Read(one[:]); err != io.EOF {
		t.Fatal("fixture server terminal", err)
	}
}

// routeBudgetReserved reads the actual independently persisted Hosting state.
func routeBudgetReserved(root string) (uint64, error) {
	raw, err := os.ReadFile(filepath.Join(root, "budget.json"))
	if err != nil {
		return 0, err
	}
	var envelope struct {
		State  json.RawMessage `json:"state"`
		SHA256 string          `json:"sha256"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, err
	}
	digest := sha256.Sum256(envelope.State)
	if hex.EncodeToString(digest[:]) != envelope.SHA256 {
		return 0, errors.New("Hosting budget checksum differs")
	}
	var state struct {
		Reserved uint64 `json:"reserved"`
	}
	if err := json.Unmarshal(envelope.State, &state); err != nil {
		return 0, err
	}
	return state.Reserved, nil
}
