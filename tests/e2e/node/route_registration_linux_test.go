package node_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	binenc "encoding/binary"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRoutePrefixRejectsSpentStock exercises an actual holder, four receiving
// processes, and an independent Issuer command in the signed Route Domain 3.
func TestRoutePrefixRejectsSpentStock(t *testing.T) {
	testRouteRegistration(t, false, false)
}

func TestRouteRegistrationRejectsSpentStock(t *testing.T) {
	testRouteRegistration(t, true, false)
}

func TestRouteRegistrationAfterReplenishment(t *testing.T) {
	testRouteRegistration(t, true, true)
}

func TestRouteRegistrationRejectsLostHistory(t *testing.T) {
	testRouteRegistration(t, true, false, "lost-history")
}

func TestRouteWithdrawnSlotRefusesAfterReceiverRestart(t *testing.T) {
	testRouteRegistration(t, true, false, "restart-withdrawn")
}

func TestRouteWithdrawalWithPendingDeliveryRetainsFailure(t *testing.T) {
	testRouteRegistration(t, true, false, "withdraw-pending")
}

func TestRoutePrefixCloseRetiresRegistration(t *testing.T) {
	testRouteRegistration(t, true, false, "prefix-close")
}

func TestRouteRegistrationRejectsClockLossAfterSpend(t *testing.T) {
	testRouteRegistration(t, true, false, "clock-after-spend")
}

func TestRouteRegistrationRejectsCancelledOpening(t *testing.T) {
	for _, mode := range []string{"cancel-after-presentation", "cancel-after-spend"} {
		t.Run(mode, func(t *testing.T) { testRouteRegistration(t, true, false, mode) })
	}
}

func TestRouteWithdrawalRetainsCallerCancellation(t *testing.T) {
	for _, mode := range []string{"withdraw-after-observation", "withdraw-at-stop"} {
		t.Run(mode, func(t *testing.T) { testRouteRegistration(t, true, false, mode) })
	}
}

func TestRouteRegistrationRetiresOnDeadline(t *testing.T) {
	testRouteRegistration(t, true, false, "registration-expiry")
}

func TestRouteRegistrationRetiresOnCallerCancellation(t *testing.T) {
	testRouteRegistration(t, true, false, "cancel-registration")
}

func TestRouteRegistrationRetiresOnClockLoss(t *testing.T) {
	testRouteRegistration(t, true, false, "clock-loss")
}

func TestRoutePrefixRetiresOnClockLoss(t *testing.T) {
	testRouteRegistration(t, false, false, "clock-loss")
}

func TestRoutePrefixRetiresOnExpiry(t *testing.T) {
	testRouteRegistration(t, false, false, "expiry")
}

func TestRoutePrefixRetiresOnHolderInterrupt(t *testing.T) {
	testRouteRegistration(t, false, false, "interrupt")
}

func TestRoutePrefixRejectsExcludedParticipant(t *testing.T) {
	testRouteRegistration(t, false, false, "excluded")
}

func TestRoutePrefixRefusesClockLossBeforeSpend(t *testing.T) {
	testRouteRegistration(t, false, false, "pre-clock-loss")
}

func testRouteRegistration(t *testing.T, registration, refill bool, special ...string) {
	t.Helper()
	mode := ""
	if len(special) != 0 {
		mode = special[0]
	}
	binary := buildNodeCommand(t)
	processfixture.WaitAdmissionHour(t)
	spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, processfixture.TempDir(t))
	spec.Nodes = spec.Nodes[:1]
	listeners := make(map[[32]byte]net.Listener)
	certificates := make(map[[32]byte]tls.Certificate)
	last := byte(16)
	domain := byte(3)
	role := admission.AllocationUser
	if registration {
		last, domain, role = 17, 4, admission.AllocationPublisher
	}
	for i := byte(12); i < last; i++ {
		subrole := byte(1)
		if i >= 14 {
			subrole = 2
		}
		if i == 16 {
			subrole = 3
		}
		processfixture.AddReceivingFixtureNode(t, &spec, listeners, certificates, epoch.CarrierClosedTCP, i, domain, subrole)
	}
	if mode == "restart-withdrawn" || mode == "withdraw-pending" {
		for i := byte(21); i <= 24; i++ {
			subrole := byte(1)
			if i >= 23 {
				subrole = 2
			}
			processfixture.AddReceivingFixtureNode(t, &spec, listeners, certificates, epoch.CarrierClosedTCP, i, 1, subrole)
		}
		processfixture.AddReceivingFixtureNode(t, &spec, listeners, certificates, epoch.CarrierClosedTCP, 25, 2, 5)
	}
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	networkinputs.PrepareIssuer(t, &issuerPlan, inventory, spec, bundle.Nodes[0], sha256.Sum256(bundle.Profile))
	clock, stopClock := networkinputs.StartObservedClock(t)
	var clockStopOnce sync.Once
	defer clockStopOnce.Do(stopClock)
	var authorityKey [32]byte
	copy(authorityKey[:], spec.Authority.Public().(ed25519.PublicKey))
	seed := func() state.AuthorityDeclaration {
		t.Helper()
		return networkinputs.SeedNetworkState(t, spec.NetworkID, authorityKey, bundle, clock)
	}
	var peers []*routeBootstrapPeer
	var budgets []string
	var spendPaths []string
	var initialSpends [][]byte
	var clockLossPaths []string
	var clockLossSpends [][]byte
	var pendingDeliverySpend []byte
	var introductionRoot, introductionConfig, introductionSpend string
	for i := byte(12); i < last; i++ {
		id := [32]byte{i}
		certPath, keyPath := processfixture.WriteNativeCredential(t, certificates[id])
		budget := processfixture.ProcessBudget(t)
		budgets = append(budgets, budget)
		spendRoot := processfixture.TempDir(t)
		plan := map[string]any{"network": seed(), "node_id": id, "spend_root": spendRoot, "hosting_root": budget,
			"certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20},
			"termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		if refill && i != 16 {
			plan["work"] = hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}
		}
		if i == 16 {
			root := processfixture.TempDir(t)
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			plan["introduction_root"] = root
			introductionRoot = root
			introductionSpend = filepath.Join(spendRoot, "closed-token-spends")
		}
		if err := listeners[id].Close(); err != nil {
			t.Fatal(err)
		}
		path := processfixture.WriteConfig(t, plan)
		if i == 16 {
			introductionConfig = path
		}
		peers = append(peers, startRouteBootstrapPeer(t, binary, path))
		if mode == "clock-loss" {
			clockLossPaths = append(clockLossPaths, filepath.Join(spendRoot, "closed-token-spends"))
		}
		if !registration && (mode == "" || mode == "interrupt" || mode == "excluded" || mode == "pre-clock-loss") {
			spendPath := filepath.Join(spendRoot, "closed-token-spends")
			initial, err := os.ReadFile(spendPath)
			if err != nil {
				t.Fatal("initial Forward spending journal", err)
			}
			spendPaths, initialSpends = append(spendPaths, spendPath), append(initialSpends, initial)
		}
	}
	if mode == "restart-withdrawn" || mode == "withdraw-pending" {
		for i := byte(21); i <= 24; i++ {
			id := [32]byte{i}
			certPath, keyPath := processfixture.WriteNativeCredential(t, certificates[id])
			budget := processfixture.ProcessBudget(t)
			budgets = append(budgets, budget)
			plan := map[string]any{"network": seed(), "node_id": id, "spend_root": processfixture.TempDir(t), "hosting_root": budget,
				"certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20},
				"termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			if err := listeners[id].Close(); err != nil {
				t.Fatal(err)
			}
			peers = append(peers, startRouteBootstrapPeer(t, binary, processfixture.WriteConfig(t, plan)))
		}
	}
	networkPlan := seed()
	var holderClock string
	if mode == "pre-clock-loss" {
		var stopHolderClock func()
		holderClock, stopHolderClock = networkinputs.StartObservedClock(t)
		stopHolderClock()
		networkPlan.ClockObservationFile = holderClock
	}
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
	route := map[string]any{"entry_root": filepath.Join(processfixture.TempDir(t), "entry"), "interior_root": filepath.Join(processfixture.TempDir(t), "interior"),
		"hosting_root": budget, "domain": domain, "deadline": time.Now().Add(30 * time.Second).UTC().Truncate(time.Second),
		"work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	if refill {
		route["work"] = hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}
	}
	if mode == "expiry" {
		route["deadline"] = time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
	}
	if mode == "cancel-registration" || mode == "registration-expiry" || mode == "withdraw-after-observation" || mode == "withdraw-at-stop" || mode == "cancel-after-presentation" || mode == "cancel-after-spend" || mode == "clock-after-spend" {
		authority, err := funding.OpenAdmissionAuthority("", &networkPlan)
		if err != nil {
			t.Fatal("holder authority", err)
		}
		defer authority.Close()
		holderRoot := processfixture.TempDir(t)
		var armed, triggered atomic.Bool
		clockFailureDone := make(chan struct{})
		var clockFailure error
		var cancelOpening context.CancelFunc
		var presentationSize int64
		var receivingBefore []byte
		var presentationMu sync.Mutex
		var originalPresentation stock.Presentation
		observe := func() (admission.AuthorityFacts, time.Time, error) {
			facts, now, err := authority.ObserveFacts()
			if err == nil && mode == "cancel-after-presentation" && armed.Load() {
				info, statErr := os.Stat(filepath.Join(holderRoot, "attempts"))
				if statErr != nil {
					return facts, now, statErr
				}
				if info.Size() > presentationSize && triggered.CompareAndSwap(false, true) {
					cancelOpening()
				}
			}
			return facts, now, err
		}
		current := func() (network.RuntimeView, error) {
			view, err := authority.Current()
			if err == nil && (mode == "cancel-after-spend" || mode == "clock-after-spend") && armed.Load() && !triggered.Load() {
				spent, readErr := os.ReadFile(introductionSpend)
				if readErr != nil {
					return view, readErr
				}
				committed, err := committedRouteSpend(receivingBefore, spent)
				if err != nil {
					return view, err
				}
				if committed && triggered.CompareAndSwap(false, true) {
					if mode == "cancel-after-spend" {
						cancelOpening()
					} else {
						clockStopOnce.Do(stopClock)
						clockFailure = os.Remove(clock)
						close(clockFailureDone)
					}
				}
			}
			if mode == "clock-after-spend" && armed.Load() && triggered.Load() {
				<-clockFailureDone
				if clockFailure != nil {
					return view, clockFailure
				}
				return authority.Current()
			}
			return view, err
		}
		present := func(p stock.Presentation) error {
			if err := authority.Presentation(p); err != nil {
				return err
			}
			if p.RecipientNodeID == [32]byte{16} {
				presentationMu.Lock()
				originalPresentation = p
				presentationMu.Unlock()
			}
			return nil
		}
		holder, err := stock.Open(holderRoot, role, observe)
		if err != nil {
			t.Fatal("holder Stock", err)
		}
		defer holder.Close()
		request, digest, err := holder.Request([3]uint32{0, 4, 1})
		if err != nil {
			t.Fatal("holder request", err)
		}
		if err := holder.Import(digest, processfixture.FixturePermission(t, request, facts, spec.Authority, now)); err != nil {
			t.Fatal("holder permission", err)
		}
		var forward []token.ClosedTokenContext
		for i := byte(12); i < 16; i++ {
			forward = append(forward, token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
				ReceiverNodeID: [32]byte{i}, ReceiverDutyGeneration: 9, Class: 2, WindowStart: now.UTC().Truncate(time.Hour)})
		}
		registrationToken := token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
			ReceiverNodeID: [32]byte{16}, ReceiverDutyGeneration: 9, Class: 3, WindowStart: now.UTC().Truncate(time.Hour)}
		for i, challenges := range [][]token.ClosedTokenContext{forward, {registrationToken}} {
			attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{72, byte(i)}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter})
			if err != nil {
				t.Fatal("holder issuance", err)
			}
			batch, _, err := attempt.Request()
			if err != nil {
				t.Fatal("issuance batch", err)
			}
			issued := admissionLocalCommand(t, binary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch, "kind": quota.Bootstrap})
			if issued.Outcome != "issued-offline" {
				t.Fatal("issuer", issued.Outcome)
			}
			if err := attempt.Complete(issued.Response, nil); err != nil {
				t.Fatal("issuance result", err)
			}
		}
		plan := funding.Plan{EntryRoot: route["entry_root"].(string), InteriorRoot: route["interior_root"].(string), HostingRoot: budget,
			Domain: domain, Deadline: route["deadline"].(time.Time), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		prefix, err := session.OpenPrefix(t.Context(), plan, funding.Observe(current, authority.Witness, present), holder)
		if err != nil {
			t.Fatal("Publisher Prefix", err)
		}
		defer prefix.Close()
		before, err := os.ReadFile(introductionSpend)
		if err != nil {
			t.Fatal("Introduction spending", err)
		}
		var caller context.Context
		var cancel context.CancelFunc
		var registrationEnd time.Time
		if mode == "registration-expiry" {
			registrationEnd = time.Now().Add(4 * time.Second).UTC().Truncate(time.Second)
			caller, cancel = context.WithDeadline(t.Context(), registrationEnd)
		} else {
			caller, cancel = context.WithCancel(t.Context())
		}
		defer cancel()
		if mode == "cancel-after-presentation" || mode == "cancel-after-spend" {
			info, err := os.Stat(filepath.Join(holderRoot, "attempts"))
			if err != nil {
				t.Fatal("holder presentation journal", err)
			}
			presentationSize = info.Size()
			receivingBefore = before
			cancelOpening = cancel
			delayed := &e2eDeferredCaller{Context: caller}
			armed.Store(true)
			result, registerErr := prefix.Register(delayed, 1)
			armed.Store(false)
			if result.Owner() != nil {
				_ = result.Owner().Close()
				t.Fatal("cancelled registration was accepted")
			}
			if !triggered.Load() || !errors.Is(registerErr, context.Canceled) || delayed.stopped.Load() != 1 {
				t.Fatal("cancelled registration lost its exact boundary or callback join", triggered.Load(), registerErr, delayed.stopped.Load())
			}
			spending, err := os.ReadFile(introductionSpend)
			if err != nil {
				t.Fatal("Introduction spending", err)
			}
			if mode == "cancel-after-presentation" {
				if !bytes.Equal(before, spending) {
					t.Fatal("cancelled presentation reached receiving spend")
				}
				presentationMu.Lock()
				presented := originalPresentation
				presentationMu.Unlock()
				if presented.RecipientNodeID != [32]byte{16} {
					t.Fatal("missing original presentation")
				}
				raw, replayErr := holder.Take(t.Context(), presented, 3)
				clear(raw)
				if replayErr == nil {
					t.Fatal("cancelled presentation restored holder Stock")
				}
			} else if bytes.Equal(before, spending) {
				t.Fatal("cancellation preceded real receiving spend")
			}
			if err := errors.Join(prefix.Close(), holder.Close(), authority.Close()); err != nil {
				t.Fatal("joined holder", err)
			}
			finishRouteJoinPeers(t, peers, budgets)
			return
		}
		if mode == "clock-after-spend" {
			receivingBefore = before
			armed.Store(true)
			result, registerErr := prefix.Register(caller, 1)
			if !triggered.Load() {
				if result.Owner() != nil {
					_ = result.Owner().Close()
				}
				t.Fatal("registration did not reach clock-loss boundary", registerErr)
			}
			<-clockFailureDone
			armed.Store(false)
			if clockFailure != nil {
				t.Fatal("remove observed clock", clockFailure)
			}
			if result.Owner() != nil {
				_ = result.Owner().Close()
				t.Fatal("registration accepted after clock loss")
			}
			const clockRefusal = "clock confidence is outside the two-second bound"
			if _, err := authority.Current(); err == nil || err.Error() != clockRefusal {
				t.Fatal("original Network State did not report the exact clock refusal", err)
			}
			// The holder watcher may interrupt the private context, or a
			// receiver observing the same loss may close the transport first.
			// The clock observation above and the original receivers' joined
			// terminal reports below establish the cause independently of
			// which failure crosses this stream first.
			if caller.Err() != nil || !processfixture.OnlyFailureBranches(registerErr, func(branch error) bool {
				return branch == context.Canceled || branch == io.EOF || branch.Error() == clockRefusal
			}) {
				t.Fatal("registration did not refuse on clock loss", registerErr, caller.Err())
			}
			spent, err := os.ReadFile(introductionSpend)
			committed, spendErr := committedRouteSpend(before, spent)
			if err != nil || spendErr != nil || !committed {
				t.Fatal("clock loss did not retain one committed receiving spend", err, spendErr)
			}
			first := prefix.Close()
			if first == nil || prefix.Close() != first {
				t.Fatal("Prefix lost retained failure", first)
			}
			for _, peer := range peers {
				done := make(chan error, 1)
				go func() { done <- peer.command.Wait() }()
				select {
				case waitErr := <-done:
					peer.waited = true
					var exit *exec.ExitError
					if !errors.As(waitErr, &exit) || exit.ExitCode() != 1 {
						t.Fatal("receiver survived clock loss", waitErr, peer.diagnostic.String())
					}
					var terminal map[string]string
					for _, line := range bytes.Split(peer.diagnostic.Bytes(), []byte{'\n'}) {
						var event map[string]string
						if json.Unmarshal(line, &event) == nil && event["operation"] == "route.receive" && event["phase"] == "joined" {
							terminal = event
						}
					}
					if terminal["outcome"] != "failed" || terminal["stage"] != "terminal" {
						t.Fatal("receiver did not retain the terminal authority failure", terminal)
					}
				case <-time.After(3 * time.Second):
					_ = peer.command.Process.Kill()
					<-done
					peer.waited = true
					t.Fatal("receiver did not join after clock loss")
				}
			}
			if err := os.WriteFile(clock, nil, 0600); err != nil {
				t.Fatal("restore clock observation", err)
			}
			stopRestoredClock := networkinputs.RefreshObservedClock(t, clock)
			defer stopRestoredClock()
			if _, _, err := authority.ObserveFacts(); err != nil {
				t.Fatal("restored holder authority", err)
			}
			presentationMu.Lock()
			presented := originalPresentation
			presentationMu.Unlock()
			if presented.RecipientNodeID != [32]byte{16} {
				t.Fatal("missing spent registration presentation")
			}
			raw, replayErr := holder.Take(t.Context(), presented, 3)
			clear(raw)
			if stock.TransferFailureStage(replayErr) != "stock" {
				t.Fatal("holder restored or masked the spent registration right", replayErr)
			}
			if err := errors.Join(holder.Close(), authority.Close()); err != nil {
				t.Fatal("holder joined result", err)
			}
			reopened := startRouteBootstrapPeer(t, binary, introductionConfig)
			peers = append(peers, reopened)
			after, err := os.ReadFile(introductionSpend)
			if err != nil || !bytes.Equal(spent, after) {
				t.Fatal("reopen changed spent registration", err)
			}
			finishRouteJoinPeers(t, peers, budgets)
			return
		}
		registration, err := prefix.Register(caller, 1)
		if err != nil {
			t.Fatal("genuine registration", err)
		}
		owner := registration.Owner()
		defer owner.Close()
		if registration.Facts().Slot == [32]byte{} {
			t.Fatal("missing registration slot")
		}
		spent, err := os.ReadFile(introductionSpend)
		if err != nil || bytes.Equal(before, spent) {
			t.Fatal("registration did not spend", err)
		}
		if mode == "registration-expiry" {
			if !registration.Facts().Expiry.Equal(registrationEnd) {
				t.Fatal("registration changed caller deadline", registration.Facts().Expiry, registrationEnd)
			}
			if !time.Now().Before(registrationEnd.Add(-200 * time.Millisecond)) {
				t.Fatal("setup consumed the live expiry observation window")
			}
			select {
			case <-owner.Done():
				t.Fatal("registration retired before its deadline")
			case <-time.After(time.Until(registrationEnd.Add(-200 * time.Millisecond))):
			}
			select {
			case <-owner.Done():
			case <-time.After(time.Until(registrationEnd) + 2*time.Second):
				t.Fatal("registration did not retire after deadline")
			}
		} else if mode == "cancel-registration" {
			cancel()
		} else {
			withdrawContext, cancelWithdraw := context.WithCancel(t.Context())
			defer cancelWithdraw()
			delayed := &e2eDeferredCaller{Context: withdrawContext}
			var withdrawCaller context.Context = &e2eWithdrawCancelAfterObservation{e2eDeferredCaller: delayed, cancel: cancelWithdraw}
			if mode == "withdraw-at-stop" {
				withdrawCaller = &e2eWithdrawCancelAtStop{e2eDeferredCaller: delayed, cancel: cancelWithdraw}
			}
			withdrawErr := owner.Withdraw(withdrawCaller)
			retained := owner.Close()
			if !errors.Is(withdrawErr, context.Canceled) || !errors.Is(retained, context.Canceled) || delayed.stopped.Load() != 1 {
				t.Fatal("WITHDRAW lost original caller cancellation or joined callback", withdrawErr, retained, delayed.stopped.Load())
			}
		}
		first := owner.Close()
		if mode == "cancel-registration" && !errors.Is(first, context.Canceled) {
			t.Fatal("joined registration lost caller cancellation", first)
		}
		if mode == "registration-expiry" && first == nil {
			t.Fatal("expired registration reported graceful completion")
		}
		if again := owner.Close(); again != first {
			t.Fatal("registration replaced retained terminal result", first, again)
		}
		select {
		case <-owner.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("cancelled registration did not join")
		}
		after, err := os.ReadFile(introductionSpend)
		if err != nil || !bytes.Equal(spent, after) {
			t.Fatal("cancelled registration changed durable spend", err)
		}
		if err := errors.Join(prefix.Close(), holder.Close(), authority.Close()); err != nil {
			t.Fatal("holder joined result", err)
		}
		finishRouteJoinPeers(t, peers, budgets)
		return
	}
	holderRoot := processfixture.TempDir(t)
	holderPlan := map[string]any{"root": holderRoot, "network": networkPlan, "role": role, "route": route}
	var send func(any) localAdmissionReply
	var closeHolder, interruptHolder func() error
	if mode == "interrupt" {
		send, closeHolder, interruptHolder = admissionLocalConsoleInterrupt(t, binary, "holder", holderPlan)
	} else {
		send, closeHolder = admissionLocalConsoleResult(t, binary, "holder", holderPlan)
	}
	maxima := [3]uint32{0, 4, 0}
	if refill {
		maxima[1] = 8
	}
	if registration {
		maxima[2] = 1
	}
	request := send(holderCommand{Operation: "request", Maxima: maxima})
	if request.Outcome != "completed" {
		t.Fatal("holder request", request.Outcome)
	}
	permission := processfixture.FixturePermission(t, request.Request, facts, spec.Authority, now)
	if reply := send(holderCommand{Operation: "import", Digest: request.Digest, Payload: permission}); reply.Outcome != "completed" {
		t.Fatal("holder import", reply.Outcome)
	}
	var challenges []token.ClosedTokenContext
	for i := byte(12); i < 16; i++ {
		challenges = append(challenges, token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest,
			IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: [32]byte{i}, ReceiverDutyGeneration: 9, Class: 2,
			WindowStart: time.Now().UTC().Truncate(time.Hour)})
		if refill {
			challenges = append(challenges, challenges[len(challenges)-1])
		}
	}
	batches := [][]token.ClosedTokenContext{challenges}
	if registration {
		batches = append(batches, []token.ClosedTokenContext{{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest,
			IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: [32]byte{16}, ReceiverDutyGeneration: 9, Class: 3,
			WindowStart: time.Now().UTC().Truncate(time.Hour)}})
	}
	for index, contexts := range batches {
		intent := stock.IssuanceIntent{Challenges: contexts, Selection: stock.ExchangeBinding{ID: [32]byte{byte(31 + index)}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter}
		batch := send(holderCommand{Operation: "begin", Intent: intent})
		if batch.Outcome != "completed" {
			t.Fatal("holder begin", batch.Outcome)
		}
		issued := admissionLocalCommand(t, binary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch.Request, "kind": quota.Bootstrap})
		if issued.Outcome != "issued-offline" {
			t.Fatal("issuer response", issued.Outcome)
		}
		if reply := send(holderCommand{Operation: "complete", Payload: issued.Response}); reply.Outcome != "completed" {
			t.Fatal("holder complete", reply.Outcome)
		}
	}
	if mode == "pre-clock-loss" {
		var initialBudgets [][]byte
		for _, root := range budgets {
			raw, err := os.ReadFile(filepath.Join(root, "budget.json"))
			if err != nil {
				t.Fatal("original Hosting budget", err)
			}
			initialBudgets = append(initialBudgets, raw)
		}
		if err := os.Remove(holderClock); err != nil {
			t.Fatal(err)
		}
		if reply := send(holderCommand{Operation: "prefix-open"}); reply.Outcome != "refused" {
			t.Fatal("Prefix accepted unavailable clock", reply.Outcome)
		}
		for i, path := range spendPaths {
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, initialSpends[i]) {
				t.Fatal("clock refusal changed receiving Admission", i, err)
			}
		}
		for i, root := range budgets {
			current, err := os.ReadFile(filepath.Join(root, "budget.json"))
			if err != nil || !bytes.Equal(current, initialBudgets[i]) {
				t.Fatal("clock refusal changed Hosting budget", i, err)
			}
		}
		if err := os.WriteFile(holderClock, nil, 0600); err != nil {
			t.Fatal(err)
		}
		stopRestoredClock := networkinputs.RefreshObservedClock(t, holderClock)
		defer stopRestoredClock()
	}
	if reply := send(holderCommand{Operation: "prefix-open"}); reply.Outcome != "completed" {
		t.Fatal("prefix-open", reply.Outcome, reply.Stage)
	}
	var spent [][]byte
	selectedEntry := -1
	if len(spendPaths) != 0 {
		var changed [2]int
		for i, path := range spendPaths {
			current, err := os.ReadFile(path)
			if err != nil {
				t.Fatal("Forward spending journal", err)
			}
			spent = append(spent, current)
			if !bytes.Equal(initialSpends[i], current) {
				changed[i/2]++
				if i < 2 {
					selectedEntry = i
				}
			}
		}
		if changed != [2]int{1, 1} {
			t.Fatal("Prefix did not spend one Entry and one Interior right", changed)
		}
	}
	if mode == "clock-loss" && !registration {
		for _, path := range clockLossPaths {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal("spend before clock loss", err)
			}
			clockLossSpends = append(clockLossSpends, raw)
		}
		clockStopOnce.Do(stopClock)
		if err := os.Remove(clock); err != nil {
			t.Fatal(err)
		}
		first := send(holderCommand{Operation: "prefix-close"})
		if first.Outcome != "refused" {
			t.Fatal("Prefix survived lost Network clock", first.Outcome)
		}
		again := send(holderCommand{Operation: "prefix-close"})
		if again.Outcome != first.Outcome || again.Stage != first.Stage {
			t.Fatal("repeated Prefix close changed its terminal refusal", first, again)
		}
		if reply := send(holderCommand{Operation: "prefix-open"}); reply.Outcome == "completed" {
			t.Fatal("Prefix reopened after authority loss")
		}
	}
	if mode == "expiry" {
		timer := time.NewTimer(time.Until(route["deadline"].(time.Time)) + 100*time.Millisecond)
		select {
		case <-timer.C:
		case <-t.Context().Done():
			timer.Stop()
			t.Fatal("waiting for original Prefix expiry", t.Context().Err())
		}
		first := send(holderCommand{Operation: "prefix-close"})
		if first.Outcome != "refused" {
			t.Fatal("expired Prefix closed as healthy", first.Outcome)
		}
		again := send(holderCommand{Operation: "prefix-close"})
		if again.Outcome != first.Outcome || again.Stage != first.Stage {
			t.Fatal("repeated expired Prefix close changed its result", first, again)
		}
	}
	var interruptedHolder error
	if mode == "interrupt" {
		interruptedHolder = interruptHolder()
		var exit *exec.ExitError
		if !errors.As(interruptedHolder, &exit) || exit.ExitCode() != 1 {
			t.Fatal("interrupted holder lost its retained Prefix failure", interruptedHolder)
		}
	}
	if registration {
		if refill {
			if reply := send(holderCommand{Operation: "prefix-replenish"}); reply.Outcome != "completed" {
				t.Fatal("prefix replenishment", reply)
			}
		}
		reply := send(holderCommand{Operation: "registration-open", Revision: 1})
		registrationFacts := reply.Registration
		if reply.Outcome != "completed" || reply.Slot == [32]byte{} || registrationFacts.Slot != reply.Slot ||
			registrationFacts.Network != facts.NetworkID || registrationFacts.Profile != facts.Digest || registrationFacts.Node != [32]byte{16} ||
			registrationFacts.Revision != 1 || registrationFacts.Created.IsZero() || !registrationFacts.Created.Before(time.Now()) ||
			!registrationFacts.Created.Before(registrationFacts.Expiry) || registrationFacts.Expiry.After(route["deadline"].(time.Time)) ||
			registrationFacts.Acknowledgement == [32]byte{} {
			t.Fatal("registration binding differs from signed state and accepted receipt", reply.Outcome)
		}
		if mode == "clock-loss" {
			for _, path := range clockLossPaths {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal("spend before clock loss", err)
				}
				clockLossSpends = append(clockLossSpends, raw)
			}
			clockStopOnce.Do(stopClock)
			if err := os.Remove(clock); err != nil {
				t.Fatal(err)
			}
			first := send(holderCommand{Operation: "registration-close"})
			if first.Outcome != "refused" {
				t.Fatal("registration survived lost Network clock", first.Outcome)
			}
			again := send(holderCommand{Operation: "registration-close"})
			if again.Outcome != first.Outcome || again.Stage != first.Stage {
				t.Fatal("registration replaced its terminal refusal", first, again)
			}
			if reply := send(holderCommand{Operation: "prefix-close"}); reply.Outcome != "refused" {
				t.Fatal("Prefix survived lost Network clock", reply.Outcome)
			}
		} else if mode == "prefix-close" {
			if reply := send(holderCommand{Operation: "prefix-close"}); reply.Outcome != "completed" {
				t.Fatal("prefix-close with active registration", reply)
			}
			if reply := send(holderCommand{Operation: "registration-withdraw"}); reply.Outcome == "completed" {
				t.Fatal("registration survived its Prefix")
			}
		} else {
			if mode != "withdraw-pending" {
				if reply := send(holderCommand{Operation: "registration-withdraw"}); reply.Outcome != "completed" {
					t.Fatal("registration withdraw", reply)
				}
			}
			if mode == "restart-withdrawn" || mode == "withdraw-pending" {
				historyPath := filepath.Join(introductionRoot, "closed-introduction-slots")
				history, err := os.ReadFile(historyPath)
				if err != nil || len(history) == 0 {
					t.Fatal("withdrawn Introduction history", err)
				}
				// An independently funded Source submits to the actual withdrawn
				// slot, before and after the receiving process is reopened.
				sourceRoot := processfixture.TempDir(t)
				sourceBudget := processfixture.ProcessBudget(t)
				budgets = append(budgets, sourceBudget)
				sourceRoute := map[string]any{"entry_root": filepath.Join(sourceRoot, "entry"), "interior_root": filepath.Join(sourceRoot, "interior"),
					"hosting_root": sourceBudget, "domain": 1, "deadline": route["deadline"], "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20},
					"termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				sourcePlan := map[string]any{"root": processfixture.TempDir(t), "network": seed(), "role": admission.AllocationUser, "route": sourceRoute}
				source, closeSource := admissionLocalConsoleResult(t, binary, "holder", sourcePlan)
				request := source(holderCommand{Operation: "request", Maxima: [3]uint32{2, 4, 0}})
				if request.Outcome != "completed" {
					t.Fatal("Source request", request.Outcome)
				}
				permission := processfixture.FixturePermission(t, request.Request, facts, spec.Authority, time.Now())
				if reply := source(holderCommand{Operation: "import", Digest: request.Digest, Payload: permission}); reply.Outcome != "completed" {
					t.Fatal("Source permission", reply.Outcome)
				}
				var sourceForward []token.ClosedTokenContext
				for i := byte(21); i <= 24; i++ {
					sourceForward = append(sourceForward, token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest,
						IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: [32]byte{i}, ReceiverDutyGeneration: 9, Class: 2,
						WindowStart: time.Now().UTC().Truncate(time.Hour)})
				}
				control := token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
					ReceiverNodeID: [32]byte{16}, ReceiverDutyGeneration: 9, Class: 1, WindowStart: time.Now().UTC().Truncate(time.Hour)}
				for index, contexts := range [][]token.ClosedTokenContext{sourceForward, {control, control}} {
					intent := stock.IssuanceIntent{Challenges: contexts, Selection: stock.ExchangeBinding{ID: [32]byte{91, byte(index)}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter}
					batch := source(holderCommand{Operation: "begin", Intent: intent})
					if batch.Outcome != "completed" {
						t.Fatal("Source issuance request", batch.Outcome)
					}
					issued := admissionLocalCommand(t, binary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch.Request, "kind": quota.Bootstrap})
					if issued.Outcome != "issued-offline" {
						t.Fatal("Source issuance", issued.Outcome)
					}
					if reply := source(holderCommand{Operation: "complete", Payload: issued.Response}); reply.Outcome != "completed" {
						t.Fatal("Source stock", reply.Outcome)
					}
				}
				if reply := source(holderCommand{Operation: "prefix-open"}); reply.Outcome != "completed" {
					t.Fatal("Source Prefix", reply.Outcome)
				}
				capsule := make([]byte, 474)
				copy(capsule[:32], registrationFacts.Slot[:])
				binenc.BigEndian.PutUint64(capsule[32:40], registrationFacts.Revision)
				expiry := time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)
				if registrationFacts.Expiry.Before(expiry) {
					expiry = registrationFacts.Expiry
				}
				binenc.BigEndian.PutUint64(capsule[40:48], uint64(expiry.Unix()))
				capsule[48], capsule[80], capsule[114] = 30, 40, 50
				binenc.BigEndian.PutUint16(capsule[112:114], 360)
				submit := func() []byte {
					t.Helper()
					before, err := os.ReadFile(introductionSpend)
					if err != nil {
						t.Fatal("Introduction spending before submission", err)
					}
					reply := source(map[string]any{"operation": "capsule-submit", "introduction": map[string]any{"node": registrationFacts.Node, "generation": uint64(9)}, "payload": capsule})
					if reply.Outcome != "refused" {
						t.Fatal("withdrawn slot accepted external submission", reply.Outcome)
					}
					after, err := os.ReadFile(introductionSpend)
					if err != nil || bytes.Equal(before, after) {
						t.Fatal("submission did not reach receiving Admission", err)
					}
					retained, err := os.ReadFile(historyPath)
					if err != nil || !bytes.Equal(retained, history) {
						t.Fatal("refused submission changed withdrawn slot history", err)
					}
					return after
				}
				if mode == "withdraw-pending" {
					before, err := os.ReadFile(introductionSpend)
					if err != nil {
						t.Fatal("Introduction spending before live delivery", err)
					}
					liveResult := make(chan localAdmissionReply, 1)
					go func() {
						var reply localAdmissionReply
						defer func() { liveResult <- reply }()
						reply = source(map[string]any{"operation": "capsule-submit", "introduction": map[string]any{"node": registrationFacts.Node, "generation": uint64(9)}, "payload": capsule})
					}()
					timer := time.NewTimer(3 * time.Second)
					ticker := time.NewTicker(10 * time.Millisecond)
					defer timer.Stop()
					defer ticker.Stop()
				waitLiveSpend:
					for {
						current, err := os.ReadFile(introductionSpend)
						if err != nil {
							t.Fatal("Introduction spending during live delivery", err)
						}
						if !bytes.Equal(before, current) {
							break waitLiveSpend
						}
						select {
						case reply := <-liveResult:
							t.Fatal("live delivery ended before Admission", reply.Outcome)
						case <-timer.C:
							t.Fatal("live delivery did not reach receiving Admission")
						case <-t.Context().Done():
							t.Fatal("waiting for live delivery", t.Context().Err())
						case <-ticker.C:
						}
					}
					select {
					case reply := <-liveResult:
						t.Fatal("live registration rejected delivery before withdrawal", reply.Outcome)
					case <-time.After(150 * time.Millisecond):
					}
					first := send(holderCommand{Operation: "registration-withdraw"})
					if first.Outcome != "refused" {
						t.Fatal("pending delivery did not retain withdrawal failure", first.Outcome)
					}
					again := send(holderCommand{Operation: "registration-close"})
					if again.Outcome != "refused" {
						t.Fatal("registration discarded withdrawal failure", again.Outcome)
					}
					select {
					case reply := <-liveResult:
						if reply.Outcome != "refused" {
							t.Fatal("withdrawn pending delivery completed", reply.Outcome)
						}
					case <-time.After(3 * time.Second):
						t.Fatal("withdrawal did not join pending delivery")
					}
					pendingDeliverySpend, err = os.ReadFile(introductionSpend)
					if err != nil || bytes.Equal(before, pendingDeliverySpend) {
						t.Fatal("pending delivery lost its irreversible spend", err)
					}
				} else {
					spending := submit()
					original := peers[4]
					if err := processfixture.Interrupt(original.command); err != nil {
						t.Fatal("interrupt Introduction receiver", err)
					}
					joined := original.command.Wait()
					original.waited = true
					var exit *exec.ExitError
					if !errors.As(joined, &exit) || exit.ExitCode() != 130 {
						t.Fatal("Introduction receiver did not join", joined, original.diagnostic.String())
					}
					peers[4] = startRouteBootstrapPeer(t, binary, introductionConfig)
					reopenedHistory, historyErr := os.ReadFile(historyPath)
					reopenedSpending, spendErr := os.ReadFile(introductionSpend)
					if historyErr != nil || spendErr != nil || !bytes.Equal(history, reopenedHistory) || !bytes.Equal(spending, reopenedSpending) {
						t.Fatal("restart changed withdrawn Introduction history or spending", historyErr, spendErr)
					}
					_ = submit()
				}
				if reply := source(holderCommand{Operation: "prefix-close"}); reply.Outcome != "completed" {
					t.Fatal("Source Prefix close", reply.Outcome)
				}
				if err := closeSource(); err != nil {
					t.Fatal("Source holder terminal", err)
				}
			}
			if reply := send(holderCommand{Operation: "registration-open", Revision: 2}); reply.Outcome == "completed" {
				t.Fatal("registration reused spent token")
			}
		}
	}
	if mode == "withdraw-pending" {
		if reply := send(holderCommand{Operation: "prefix-close"}); reply.Outcome != "completed" {
			t.Fatal("Prefix did not join after registration failure", reply.Outcome)
		}
		if reply := send(holderCommand{Operation: "registration-close"}); reply.Outcome != "refused" {
			t.Fatal("Prefix close erased registration failure", reply.Outcome)
		}
	}
	if mode != "prefix-close" && mode != "clock-loss" && mode != "expiry" && mode != "interrupt" && mode != "withdraw-pending" {
		if reply := send(holderCommand{Operation: "prefix-close"}); reply.Outcome != "completed" {
			t.Fatal("prefix-close", reply.Outcome, reply.Stage)
		}
	}
	if mode != "clock-loss" && mode != "interrupt" && mode != "excluded" && mode != "withdraw-pending" {
		if reply := send(holderCommand{Operation: "prefix-open"}); reply.Outcome == "completed" {
			t.Fatal("holder reopened Prefix using spent Forward rights")
		}
	}
	for i, path := range spendPaths {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, spent[i]) {
			t.Fatal("refused Prefix reopen changed receiving spend", i, err)
		}
	}
	holderErr := closeHolder()
	if mode == "clock-loss" || mode == "expiry" || mode == "withdraw-pending" {
		var exit *exec.ExitError
		if !errors.As(holderErr, &exit) || exit.ExitCode() != 1 {
			t.Fatal("holder lost terminal authority failure", holderErr)
		}
	} else if holderErr != nil {
		t.Fatal("holder terminal", holderErr)
	}
	if mode == "excluded" {
		if selectedEntry < 0 {
			t.Fatal("selected Entry was not observed in its spending journal")
		}
		member, err := view.Member([32]byte{byte(12 + selectedEntry)}, view.ObservedAt())
		if err != nil {
			t.Fatal("signed selected Entry", err)
		}
		openFunded := func(attemptRoute map[string]any, index byte) (func(any) localAdmissionReply, func() error) {
			t.Helper()
			attemptPlan := map[string]any{"root": processfixture.TempDir(t), "network": networkPlan, "role": role, "route": attemptRoute}
			attempt, finish := admissionLocalConsoleResult(t, binary, "holder", attemptPlan)
			request := attempt(holderCommand{Operation: "request", Maxima: [3]uint32{0, 4, 0}})
			if request.Outcome != "completed" {
				t.Fatal("fresh holder request", request.Outcome)
			}
			permission := processfixture.FixturePermission(t, request.Request, facts, spec.Authority, time.Now())
			if reply := attempt(holderCommand{Operation: "import", Digest: request.Digest, Payload: permission}); reply.Outcome != "completed" {
				t.Fatal("fresh holder permission", reply.Outcome)
			}
			intent := stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{81, index}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter}
			batch := attempt(holderCommand{Operation: "begin", Intent: intent})
			if batch.Outcome != "completed" {
				t.Fatal("fresh holder batch", batch.Outcome)
			}
			issued := admissionLocalCommand(t, binary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch.Request, "kind": quota.Bootstrap})
			if issued.Outcome != "issued-offline" {
				t.Fatal("fresh holder issuance", issued.Outcome)
			}
			if reply := attempt(holderCommand{Operation: "complete", Payload: issued.Response}); reply.Outcome != "completed" {
				t.Fatal("fresh holder stock", reply.Outcome)
			}
			return attempt, finish
		}
		var index byte
		for name, value := range map[string][32]byte{"NodeID": member.NodeID, "PublicKey": member.PublicKey, "FamilyID": member.FamilyID} {
			attemptRoute := make(map[string]any, len(route)+1)
			for key, value := range route {
				attemptRoute[key] = value
			}
			attemptRoute["exclusions"] = []map[string]any{{name: value}}
			index++
			attempt, finish := openFunded(attemptRoute, index)
			if reply := attempt(holderCommand{Operation: "prefix-open"}); reply.Outcome != "refused" {
				t.Fatal("excluded retained Entry accepted", name, reply.Outcome)
			}
			if err := finish(); err != nil {
				t.Fatal("excluded holder did not join", name, err)
			}
			for i, path := range spendPaths {
				current, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(current, spent[i]) {
					t.Fatal("excluded Prefix touched receiving Admission", name, i, err)
				}
			}
		}
		positive, finish := openFunded(route, 4)
		for _, operation := range []string{"prefix-open", "prefix-close"} {
			if reply := positive(holderCommand{Operation: operation}); reply.Outcome != "completed" {
				t.Fatal("retained selection unavailable without exclusions", operation, reply.Outcome, reply.Stage)
			}
		}
		if err := finish(); err != nil {
			t.Fatal("positive holder terminal", err)
		}
	}
	for _, peer := range peers {
		if mode == "clock-loss" {
			continue
		}
		if err := processfixture.Interrupt(peer.command); err != nil {
			t.Fatal("interrupt receiver", err)
		}
	}
	for _, peer := range peers {
		var err error
		if mode == "clock-loss" {
			joined := make(chan error, 1)
			go func() { joined <- peer.command.Wait() }()
			select {
			case err = <-joined:
			case <-time.After(3 * time.Second):
				_ = peer.command.Process.Kill()
				<-joined
				peer.waited = true
				t.Fatal("receiver failed to join after clock loss")
			}
		} else {
			err = peer.command.Wait()
		}
		peer.waited = true
		var exit *exec.ExitError
		if mode == "clock-loss" {
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatal("receiver survived clock loss", err, peer.diagnostic.String())
			}
			var terminal map[string]string
			for _, line := range bytes.Split(peer.diagnostic.Bytes(), []byte{'\n'}) {
				var event map[string]string
				if json.Unmarshal(line, &event) == nil && event["operation"] == "route.receive" && event["phase"] == "joined" {
					terminal = event
				}
			}
			if terminal["outcome"] != "failed" {
				t.Fatal("receiver lost terminal failure", terminal)
			}
			continue
		}
		if !errors.As(err, &exit) || exit.ExitCode() != 130 {
			if registration && exit != nil && exit.ExitCode() == 1 {
				var terminal map[string]string
				for _, line := range bytes.Split(peer.diagnostic.Bytes(), []byte{'\n'}) {
					var event map[string]string
					if json.Unmarshal(line, &event) == nil && event["operation"] == "route.receive" && event["phase"] == "joined" {
						terminal = event
					}
				}
				if terminal["outcome"] == "failed" && terminal["stage"] == "peer-retired-write" {
					t.Log("receiver retained joined peer-retirement write failure")
					continue
				}
			}
			t.Fatal("receiver did not join", err, peer.diagnostic.String())
		}
	}
	for _, root := range budgets {
		owner, err := hosting.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		observed, observeErr := owner.Observe(t.Context())
		closeErr := owner.Close()
		if observeErr != nil || closeErr != nil || observed.ReservedBytes != 0 {
			t.Fatal("joined receiver retained Hosting", observeErr, closeErr, observed.ReservedBytes)
		}
	}
	for i, path := range clockLossPaths {
		current, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(current, clockLossSpends[i]) {
			t.Fatal("clock loss changed durable spending", i, err)
		}
	}
	if mode == "withdraw-pending" {
		current, err := os.ReadFile(introductionSpend)
		if err != nil || !bytes.Equal(current, pendingDeliverySpend) {
			t.Fatal("pending-delivery cleanup changed retained spending", err)
		}
	}
	if mode == "lost-history" {
		for _, name := range []string{"closed-introduction-slots", ".ardents-introduction-lock"} {
			if err := os.Remove(filepath.Join(introductionRoot, name)); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Remove(introductionRoot); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(introductionRoot, 0700); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(t.Context(), binary, "route", "receive", "--config", introductionConfig)
		output, err := command.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			t.Fatal("receiver recreated lost Introduction history", err, string(output))
		}
		entries, err := os.ReadDir(introductionRoot)
		if err != nil || len(entries) != 0 {
			t.Fatal("receiver touched lost Introduction history", err, len(entries))
		}
	}
}

// e2eDeferredCaller delays only callback scheduling; the original context,
// physical receiver, Network observations and response remain genuine.
type e2eDeferredCaller struct {
	context.Context
	stopped atomic.Int32
}

func (c *e2eDeferredCaller) Value(any) any { return nil }
func (c *e2eDeferredCaller) AfterFunc(func()) func() bool {
	var once sync.Once
	return func() bool {
		stopped := false
		once.Do(func() { c.stopped.Add(1); stopped = true })
		return stopped
	}
}

type e2eWithdrawCancelAfterObservation struct {
	*e2eDeferredCaller
	cancel   context.CancelFunc
	observed atomic.Bool
}

func (c *e2eWithdrawCancelAfterObservation) Err() error {
	err := c.Context.Err()
	if err == nil && c.observed.CompareAndSwap(false, true) {
		c.cancel()
	}
	return err
}

type e2eWithdrawCancelAtStop struct {
	*e2eDeferredCaller
	cancel context.CancelFunc
}

func (c *e2eWithdrawCancelAtStop) AfterFunc(func()) func() bool {
	var once sync.Once
	return func() bool {
		stopped := false
		once.Do(func() { c.stopped.Add(1); c.cancel(); stopped = true })
		return stopped
	}
}
