package node_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// TestRouteBootstrapAndOrdinaryIssuer runs the actual receiver and holder
// commands against independently signed network state and real issuer stores.
func TestRouteBootstrapAndOrdinaryIssuer(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, false)
}

func TestRouteBootstrapRefusesLostClock(t *testing.T) {
	testRouteBootstrapAndIssuer(t, true, false, false)
}

func TestRouteIssuerLostResultDoesNotReconstructHolder(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, false, "lost-route-result")
}

func TestRouteIssuerLostResultRetriesOnLivePrefix(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, false, "live-route-retry")
}

func TestRouteIssuerStopsAfterSigning(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, false, "issuer-stop-after-signing")
}

func TestRouteIssuerStopsAfterReceivingSpend(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, false, "issuer-stop-after-receiving")
}

func TestRouteIssuerStopsAfterQuotaDebit(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, false, "issuer-stop-after-debit")
}

func TestRouteIssuerStopsBeforeReceivingSpend(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, false, "issuer-stop-before-spend")
}

func TestRouteIssuerRefusesBusyQuotaBeforeIssuance(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, false, "busy-quota")
}

func TestRouteIssuerRejectsSuccessorBeforeReceivingSpend(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, false, "state-before-spend")
}

func TestRouteIssuerRejectsSuccessorAfterSigning(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, false, "state-after-signing")
}

func TestRouteIssuerCancellationAtVerifiedDeposit(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, false, "verified-cancel")
}

func TestRouteIssuerRejectsClockLossAfterDraft(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, false, "draft-clock-loss")
}

func TestRouteSingleBootstrapIssuerJoins(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, true, false)
}

func TestRouteSequentialOrdinaryIssuerExchanges(t *testing.T) {
	testRouteBootstrapAndIssuer(t, false, false, true)
}

func testRouteBootstrapAndIssuer(t *testing.T, loseHolderClock, singleBootstrap, sequential bool, special ...string) {
	t.Helper()
	draftClockLoss := len(special) != 0 && special[0] == "draft-clock-loss"
	verifiedCancel := len(special) != 0 && special[0] == "verified-cancel"
	liveRetry := len(special) != 0 && special[0] == "live-route-retry"
	busyQuota := len(special) != 0 && special[0] == "busy-quota"
	stopIssuerBeforeSpend := len(special) != 0 && special[0] == "issuer-stop-before-spend"
	stopIssuerAfterDebit := len(special) != 0 && special[0] == "issuer-stop-after-debit"
	stopIssuerAfterReceiving := len(special) != 0 && special[0] == "issuer-stop-after-receiving"
	stopIssuerAfterSigning := len(special) != 0 && special[0] == "issuer-stop-after-signing"
	stopIssuer := stopIssuerBeforeSpend || stopIssuerAfterReceiving || stopIssuerAfterDebit || stopIssuerAfterSigning
	stateAfterSigning := len(special) != 0 && special[0] == "state-after-signing"
	stateBeforeSpend := len(special) != 0 && special[0] == "state-before-spend"
	stateTransition := stateAfterSigning || stateBeforeSpend
	lostResult := len(special) != 0 && (special[0] == "lost-route-result" || liveRetry)
	binary := buildNodeCommand(t)
	processfixture.WaitAdmissionHour(t)
	spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, processfixture.TempDir(t))
	spec.Nodes = spec.Nodes[:1]
	listeners := make(map[[32]byte]net.Listener)
	certificates := make(map[[32]byte]tls.Certificate)
	for i := byte(12); i < 16; i++ {
		subrole := byte(1)
		if i >= 14 {
			subrole = 2
		}
		processfixture.AddReceivingFixtureNode(t, &spec, listeners, certificates, epoch.CarrierClosedTCP, i, 1, subrole)
	}
	issuerID := spec.Nodes[0].NodeID
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	listeners[issuerID] = listener
	spec.Nodes[0].Endpoint = listener.Addr().String()
	certificates[issuerID] = processfixture.RoleCertificate(t, spec.Nodes[0].PrivateKey)
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
	var spendRoots []string
	var originalSpends [][]byte
	var issuerConfig string
	receiverTermination := hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}, issuerID} {
		certPath, keyPath := processfixture.WriteNativeCredential(t, certificates[id])
		budget := processfixture.ProcessBudget(t)
		budgets = append(budgets, budget)
		spendRoot := processfixture.TempDir(t)
		plan := map[string]any{"network": seed(), "node_id": id, "spend_root": spendRoot, "hosting_root": budget,
			"certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20},
			"termination": receiverTermination}
		if id == issuerID {
			plan["issuer"] = issuerPlan
		}
		if err := listeners[id].Close(); err != nil {
			t.Fatal(err)
		}
		peerConfig := processfixture.WriteConfig(t, plan)
		if id == issuerID {
			issuerConfig = peerConfig
		}
		peers = append(peers, startRouteBootstrapPeer(t, binary, peerConfig))
		spend, err := os.ReadFile(filepath.Join(spendRoot, "closed-token-spends"))
		if err != nil {
			t.Fatal("receiver spending root", err)
		}
		spendRoots = append(spendRoots, spendRoot)
		originalSpends = append(originalSpends, spend)
	}
	networkPlan := seed()
	var holderClock string
	if loseHolderClock || draftClockLoss {
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
		"hosting_root": budget, "domain": 1, "deadline": time.Now().Add(30 * time.Second).UTC().Truncate(time.Second),
		"work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	if verifiedCancel || lostResult || stateTransition || busyQuota || stopIssuer {
		var clientAuthority funding.AdmissionAuthority
		var holderState *state.State
		if stateTransition {
			holderState, err = state.OpenRetained(config)
			if err == nil {
				clientAuthority = funding.NetworkAdmissionAuthority(holderState.CurrentRuntime, holderState.Close, holderState.CurrentRuntimeWitness)
			}
		} else {
			clientAuthority, err = funding.OpenAdmissionAuthority("", &networkPlan)
		}
		if err != nil {
			t.Fatal("holder authority", err)
		}
		defer clientAuthority.Close()
		quotaPath := filepath.Join(issuerPlan.AdmissionRoot, "admission.journal")
		resultPath := filepath.Join(issuerPlan.ResultRoot, "results.journal")
		var quotaBefore, resultBefore []byte
		var armed, resultInterrupted atomic.Bool
		var postCommitObservations atomic.Int32
		var cancelIssue context.CancelFunc
		var successor networkfixture.Closed
		if stateTransition {
			nextSpec := spec
			nextSpec.Number++
			nextSpec.Previous = bundle.Epoch.Digest
			nextSpec.Seed[0]++
			successor, err = networkfixture.BuildClosed(nextSpec)
			if err != nil {
				t.Fatal("signed successor", err)
			}
		}
		var transitionErr error
		transitionDone := make(chan struct{})
		holderRoot := processfixture.TempDir(t)
		var initialAttempts int64
		var issuerSpendBefore []byte
		var presented atomic.Bool
		observe := func() (admission.AuthorityFacts, time.Time, error) {
			facts, now, err := clientAuthority.ObserveFacts()
			if err == nil && (stateBeforeSpend || stopIssuerBeforeSpend) && armed.Load() {
				info, statErr := os.Stat(filepath.Join(holderRoot, "attempts"))
				if statErr != nil {
					return facts, now, statErr
				}
				if info.Size() > initialAttempts {
					presented.Store(true)
				}
			}
			if err == nil && verifiedCancel && armed.Load() {
				quota, readErr := os.ReadFile(quotaPath)
				if readErr != nil {
					return facts, now, readErr
				}
				if !bytes.Equal(quotaBefore, quota) && postCommitObservations.Add(1) == 2 {
					cancelIssue()
				}
			}
			return facts, now, err
		}
		current := func() (network.RuntimeView, error) {
			view, err := clientAuthority.Current()
			stopReached := stopIssuerBeforeSpend && presented.Load()
			if err == nil && stopIssuerAfterReceiving && armed.Load() {
				spent, readErr := os.ReadFile(filepath.Join(spendRoots[len(spendRoots)-1], "closed-token-spends"))
				if readErr != nil {
					return view, readErr
				}
				stopReached = !bytes.Equal(issuerSpendBefore, spent)
			}
			if err == nil && stopIssuerAfterDebit && armed.Load() {
				debits, readErr := os.ReadFile(quotaPath)
				if readErr != nil {
					return view, readErr
				}
				stopReached = committedQuotaDebit(quotaBefore, debits)
			}
			if err == nil && stopIssuerAfterSigning && armed.Load() {
				results, readErr := os.ReadFile(resultPath)
				if readErr != nil {
					return view, readErr
				}
				_, _, stopReached = committedIssuerPair(resultBefore, results)
			}
			if err == nil && stopIssuer && armed.Load() && stopReached {
				if resultInterrupted.CompareAndSwap(false, true) {
					peer := peers[len(peers)-1]
					transitionErr = processfixture.Interrupt(peer.command)
					if transitionErr == nil {
						waitErr := peer.command.Wait()
						peer.waited = true
						var exit *exec.ExitError
						if !errors.As(waitErr, &exit) || exit.ExitCode() != 130 {
							transitionErr = errors.Join(errors.New("issuer did not join its original interruption"), waitErr)
						}
					}
					close(transitionDone)
				}
				<-transitionDone
				if transitionErr != nil {
					return view, transitionErr
				}
			}
			if err == nil && stateTransition && armed.Load() {
				if !resultInterrupted.Load() {
					reached := presented.Load() && stateBeforeSpend
					if stateAfterSigning {
						results, readErr := os.ReadFile(resultPath)
						if readErr != nil {
							return view, readErr
						}
						_, _, reached = committedIssuerPair(resultBefore, results)
					}
					if reached && resultInterrupted.CompareAndSwap(false, true) {
						_, transitionErr = holderState.AcceptEpoch(t.Context(), successor.Epoch.Raw, successor.Epoch.Inputs, successor.Epoch.Materials)
						if transitionErr == nil {
							_, transitionErr = holderState.AcceptClosedProfile(successor.Profile)
						}
						close(transitionDone)
					}
				}
				if resultInterrupted.Load() {
					<-transitionDone
					if transitionErr != nil {
						return view, transitionErr
					}
					return clientAuthority.Current()
				}
			}
			if err == nil && lostResult && armed.Load() && !resultInterrupted.Load() {
				results, readErr := os.ReadFile(resultPath)
				if readErr != nil {
					return view, readErr
				}
				if _, _, complete := committedIssuerPair(resultBefore, results); complete && resultInterrupted.CompareAndSwap(false, true) {
					cancelIssue()
				}
			}
			return view, err
		}
		holder, err := stock.Open(holderRoot, admission.AllocationUser, observe)
		if err != nil {
			t.Fatal("holder Stock", err)
		}
		defer holder.Close()
		request, digest, err := holder.Request([3]uint32{2, 8, 0})
		if err != nil {
			t.Fatal("holder request", err)
		}
		permission := processfixture.FixturePermission(t, request, facts, spec.Authority, now)
		if err := holder.Import(digest, permission); err != nil {
			t.Fatal("holder permission", err)
		}
		plan := funding.Plan{EntryRoot: route["entry_root"].(string), InteriorRoot: route["interior_root"].(string), HostingRoot: budget,
			Domain: 1, Deadline: route["deadline"].(time.Time), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		boot, err := session.OpenBootstrap(t.Context(), plan, funding.Observe(clientAuthority.Current, clientAuthority.Witness, clientAuthority.Presentation), holder)
		if err != nil {
			t.Fatal("genuine Bootstrap", err)
		}
		defer boot.Close()
		for _, class := range []uint8{2, 1} {
			var receivers [][32]byte
			if (liveRetry || busyQuota) && class == 1 {
				receivers = [][32]byte{issuerID, issuerID}
			}
			if err := boot.Issue(t.Context(), class, receivers); err != nil {
				t.Fatal("bootstrap issuance", class, err)
			}
		}
		if err := boot.Close(); err != nil {
			t.Fatal("bootstrap join", err)
		}
		var presentationMu sync.Mutex
		var forwarding stock.Presentation
		present := func(p stock.Presentation) error {
			if err := clientAuthority.Presentation(p); err != nil {
				return err
			}
			if p.RecipientNodeID != issuerID {
				presentationMu.Lock()
				forwarding = p
				presentationMu.Unlock()
			}
			return nil
		}
		prefix, err := session.OpenPrefix(t.Context(), plan, funding.Observe(current, clientAuthority.Witness, present), holder)
		if err != nil {
			t.Fatal("ordinary Prefix", err)
		}
		defer prefix.Close()
		quotaBefore, err = os.ReadFile(quotaPath)
		if err != nil {
			t.Fatal("original issuer quota", err)
		}
		resultBefore, err = os.ReadFile(resultPath)
		if err != nil {
			t.Fatal("original issuer results", err)
		}
		before := holder.Status()
		if busyQuota {
			lease, err := quota.Open(issuerPlan.AdmissionRoot, issuerPlan.AdmissionBinding)
			if err != nil {
				t.Fatal("hold actual issuer quota owner", err)
			}
			defer lease.Close()
			issuerSpendPath := filepath.Join(spendRoots[len(spendRoots)-1], "closed-token-spends")
			spendBefore, err := os.ReadFile(issuerSpendPath)
			if err != nil {
				t.Fatal("original issuer receiving history", err)
			}
			firstErr := prefix.Issue(t.Context(), 2, nil)
			if firstErr == nil {
				t.Fatal("issuer accepted a busy quota owner")
			}
			pending := holder.Status()
			if pending.Busy || pending.Pending || pending.Remaining[1] != before.Remaining[1]-2 || firstErr.Error() != "closed token batch is unavailable" {
				t.Fatal("busy refusal changed finite allocation or outcome", before, pending, firstErr)
			}
			quotaBusy, quotaErr := os.ReadFile(quotaPath)
			resultBusy, resultErr := os.ReadFile(resultPath)
			spendBusy, spendErr := os.ReadFile(issuerSpendPath)
			if quotaErr != nil || resultErr != nil || spendErr != nil || !bytes.Equal(quotaBefore, quotaBusy) || !bytes.Equal(resultBefore, resultBusy) || bytes.Equal(spendBefore, spendBusy) {
				t.Fatal("busy refusal changed issuance or bypassed real receiving", quotaErr, resultErr, spendErr)
			}
			// The Issuer returns work before its terminal CLOSE. That frame
			// can reach the caller while the remote termination share is
			// still held. Observe its return with the quota lease still held.
			waitIssuerTerminationReturn(t, budgets[len(spendRoots)-1], receiverTermination.Tx+receiverTermination.Rx)
			if err := lease.Close(); err != nil {
				t.Fatal("release issuer quota owner", err)
			}
			if err := prefix.Issue(t.Context(), 2, nil); err != nil {
				t.Fatal("fresh request after quota owner release", err)
			}
			after := holder.Status()
			if after.Busy || after.Pending || after.Remaining[1] != pending.Remaining[1]-2 || after.Remaining[0] != pending.Remaining[0] || after.Remaining[2] != pending.Remaining[2] {
				t.Fatal("fresh request changed finite allocation or did not finish", pending, after)
			}
			quotaAfter, quotaErr := os.ReadFile(quotaPath)
			resultAfter, resultErr := os.ReadFile(resultPath)
			spendAfter, spendErr := os.ReadFile(issuerSpendPath)
			if quotaErr != nil || resultErr != nil || spendErr != nil || bytes.Equal(quotaBusy, quotaAfter) || bytes.Equal(resultBusy, resultAfter) || bytes.Equal(spendBusy, spendAfter) {
				t.Fatal("fresh request lacked its actual effects", quotaErr, resultErr, spendErr)
			}
			if err := errors.Join(prefix.Close(), holder.Close(), clientAuthority.Close()); err != nil {
				t.Fatal("issuer retry join", err)
			}
			finishRouteJoinPeers(t, peers, budgets)
			return
		}
		if stateBeforeSpend || stopIssuer {
			info, statErr := os.Stat(filepath.Join(holderRoot, "attempts"))
			if statErr != nil {
				t.Fatal("holder attempts", statErr)
			}
			initialAttempts = info.Size()
			issuerSpendBefore, err = os.ReadFile(filepath.Join(spendRoots[len(spendRoots)-1], "closed-token-spends"))
			if err != nil {
				t.Fatal("issuer receiving history", err)
			}
		}
		caller, cancel := context.WithCancel(t.Context())
		defer cancel()
		cancelIssue = cancel
		armed.Store(true)
		issueErr := prefix.Issue(caller, 2, nil)
		armed.Store(false)
		if stopIssuer {
			if !resultInterrupted.Load() || (stopIssuerBeforeSpend && !presented.Load()) {
				t.Fatal("issuer interruption did not reach its intended boundary", issueErr)
			}
			<-transitionDone
			if transitionErr != nil || issueErr == nil {
				t.Fatal("stopped issuer accepted the original exchange", transitionErr, issueErr)
			}
			debits, debitErr := os.ReadFile(quotaPath)
			results, resultErr := os.ReadFile(resultPath)
			spendPath := filepath.Join(spendRoots[len(spendRoots)-1], "closed-token-spends")
			issuerSpend, spendErr := os.ReadFile(spendPath)
			if debitErr != nil || resultErr != nil || spendErr != nil {
				t.Fatal("stopped issuer histories", debitErr, resultErr, spendErr)
			}
			if stopIssuerBeforeSpend && (!bytes.Equal(quotaBefore, debits) || !bytes.Equal(resultBefore, results) || !bytes.Equal(issuerSpendBefore, issuerSpend)) {
				t.Fatal("stopped issuer produced a pre-restart effect")
			}
			if stopIssuerAfterReceiving && bytes.Equal(issuerSpendBefore, issuerSpend) {
				t.Fatal("issuer interruption preceded receiving spend")
			}
			if stopIssuerAfterReceiving && !bytes.Equal(quotaBefore, debits) && !committedQuotaDebit(quotaBefore, debits) {
				t.Fatal("issuer interruption left an invalid quota history")
			}
			if stopIssuerAfterReceiving && !bytes.Equal(resultBefore, results) {
				_, _, committed := committedIssuerPair(resultBefore, results)
				if !committed || !committedQuotaDebit(quotaBefore, debits) {
					t.Fatal("issuer interruption left a partial result or an uncharged issuance")
				}
			}
			if (stopIssuerAfterDebit || stopIssuerAfterSigning) && (!committedQuotaDebit(quotaBefore, debits) || bytes.Equal(issuerSpendBefore, issuerSpend)) {
				t.Fatal("issuer interruption preceded durable debit or receiving spend")
			}
			if stopIssuerAfterSigning {
				_, _, committed := committedIssuerPair(resultBefore, results)
				if !committed {
					t.Fatal("issuer interruption preceded the signed result commit")
				}
			}
			pending := holder.Status()
			if pending.Busy || !pending.Pending || pending.Remaining[1] != before.Remaining[1]-2 {
				t.Fatal("issuer interruption changed pending issuance", before, pending)
			}
			reopened := startRouteBootstrapPeer(t, binary, issuerConfig)
			peers = append(peers, reopened)
			debitsAfter, debitErr := os.ReadFile(quotaPath)
			resultsAfter, resultErr := os.ReadFile(resultPath)
			spendAfter, spendErr := os.ReadFile(spendPath)
			if debitErr != nil || resultErr != nil || spendErr != nil || !bytes.Equal(debits, debitsAfter) || !bytes.Equal(results, resultsAfter) || !bytes.Equal(issuerSpend, spendAfter) {
				t.Fatal("issuer restart changed retained histories", debitErr, resultErr, spendErr)
			}
			if stopIssuerAfterSigning {
				batch, response, ok := committedIssuerPair(resultBefore, results)
				if !ok {
					t.Fatal("committed request and result unavailable")
				}
				defer clear(batch)
				defer clear(response)
				retry := admissionLocalCommand(t, binary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch, "kind": quota.Admitted})
				if retry.Outcome != "already-issued" || !bytes.Equal(retry.Response, response) {
					t.Fatal("restarted issuer changed the committed result", retry.Outcome)
				}
				finalDebits, debitErr := os.ReadFile(quotaPath)
				finalResults, resultErr := os.ReadFile(resultPath)
				if debitErr != nil || resultErr != nil || !bytes.Equal(debits, finalDebits) || !bytes.Equal(results, finalResults) {
					t.Fatal("replayed result changed issuer histories", debitErr, resultErr)
				}
			}
			terminal := prefix.Close()
			if prefix.Close() != terminal {
				t.Fatal("Prefix replaced its retained retirement result", terminal)
			}
			if err := errors.Join(holder.Close(), clientAuthority.Close()); err != nil {
				t.Fatal("holder join", err)
			}
			finishRouteJoinPeers(t, peers, budgets)
			return
		}
		if stateBeforeSpend {
			if !resultInterrupted.Load() || !presented.Load() {
				t.Fatal("successor did not follow holder presentation", issueErr)
			}
			<-transitionDone
			if transitionErr != nil || issueErr == nil {
				t.Fatal("successor failed to refuse issuer exchange", transitionErr, issueErr)
			}
			debits, debitErr := os.ReadFile(quotaPath)
			results, resultErr := os.ReadFile(resultPath)
			issuerSpend, spendErr := os.ReadFile(filepath.Join(spendRoots[len(spendRoots)-1], "closed-token-spends"))
			if debitErr != nil || resultErr != nil || spendErr != nil || !bytes.Equal(quotaBefore, debits) || !bytes.Equal(resultBefore, results) || !bytes.Equal(issuerSpendBefore, issuerSpend) {
				t.Fatal("successor allowed receiving or issuance effects", debitErr, resultErr, spendErr)
			}
			if after := holder.Status(); after.Busy || after.Pending || after.Remaining[1] != before.Remaining[1]-2 {
				t.Fatal("successor rebound or refunded pending issuance", before, after)
			}
			terminal := prefix.Close()
			if terminal == nil || prefix.Close() != terminal {
				t.Fatal("original Prefix lost successor refusal", terminal)
			}
			if err := errors.Join(holder.Close(), clientAuthority.Close()); err != nil {
				t.Fatal("holder join", err)
			}
			finishRouteJoinPeers(t, peers, budgets)
			finalDebits, debitErr := os.ReadFile(quotaPath)
			finalResults, resultErr := os.ReadFile(resultPath)
			finalSpend, spendErr := os.ReadFile(filepath.Join(spendRoots[len(spendRoots)-1], "closed-token-spends"))
			if debitErr != nil || resultErr != nil || spendErr != nil || !bytes.Equal(quotaBefore, finalDebits) || !bytes.Equal(resultBefore, finalResults) || !bytes.Equal(issuerSpendBefore, finalSpend) {
				t.Fatal("retired issuer produced a late effect", debitErr, resultErr, spendErr)
			}
			return
		}
		if stateAfterSigning {
			if !resultInterrupted.Load() {
				t.Fatal("successor was not applied before issuer completion", issueErr)
			}
			<-transitionDone
			if transitionErr != nil || issueErr == nil {
				t.Fatal("successor did not refuse the original signed exchange", transitionErr, issueErr)
			}
			after := holder.Status()
			if after.Busy || after.Pending || after.Remaining[1] != before.Remaining[1]-2 {
				t.Fatal("successor rebound or refunded the original issuance", before, after)
			}
			results, err := os.ReadFile(resultPath)
			if err != nil {
				t.Fatal("committed issuer result", err)
			}
			_, _, committed := committedIssuerPair(resultBefore, results)
			debits, debitErr := os.ReadFile(quotaPath)
			if !committed || debitErr != nil || bytes.Equal(quotaBefore, debits) {
				t.Fatal("successor erased the original issuer commit", committed, debitErr)
			}
			select {
			case <-prefix.Done():
			case <-time.After(5 * time.Second):
				t.Fatal("original Prefix survived successor State")
			}
			terminal := prefix.Close()
			if terminal == nil || prefix.Close() != terminal {
				t.Fatal("original Prefix lost its retained refusal", terminal)
			}
			if err := errors.Join(holder.Close(), clientAuthority.Close()); err != nil {
				t.Fatal("holder join", err)
			}
			finishRouteJoinPeers(t, peers, budgets)
			finalDebits, debitErr := os.ReadFile(quotaPath)
			finalResults, resultErr := os.ReadFile(resultPath)
			if debitErr != nil || resultErr != nil || !bytes.Equal(debits, finalDebits) || !bytes.Equal(results, finalResults) {
				t.Fatal("obsolete issuance changed issuer histories after retirement", debitErr, resultErr)
			}
			return
		}
		if lostResult {
			if !resultInterrupted.Load() || !errors.Is(issueErr, context.Canceled) {
				t.Fatal("committed result did not lose its original caller", resultInterrupted.Load(), issueErr)
			}
			after := holder.Status()
			if after.Busy || !after.Pending || after.Remaining[1] != before.Remaining[1]-2 {
				t.Fatal("lost result changed pending finite allocation", before, after)
			}
			if !liveRetry {
				if err := errors.Join(prefix.Close(), holder.Close(), clientAuthority.Close()); err != nil {
					t.Fatal("joined holder", err)
				}
			}
			// The Issuer process remains live while its real Hosting reservation
			// returns; stopping the listener cannot stand in for borrower cleanup.
			issuerBudget, err := hosting.Open(budgets[len(spendRoots)-1])
			if err != nil {
				t.Fatal("open issuer Hosting", err)
			}
			deadline := time.NewTimer(3 * time.Second)
			defer deadline.Stop()
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
		waitIssuer:
			for {
				observation, err := issuerBudget.Observe(t.Context())
				if err != nil {
					t.Fatal("issuer Hosting observation", err)
				}
				if observation.ReservedBytes == 0 {
					break waitIssuer
				}
				select {
				case <-deadline.C:
					t.Fatal("issuer borrower did not return reservation")
				case <-ticker.C:
				case <-t.Context().Done():
					t.Fatal(t.Context().Err())
				}
			}
			if err := issuerBudget.Close(); err != nil {
				t.Fatal("issuer Hosting close", err)
			}
			if liveRetry {
				debits, debitErr := os.ReadFile(quotaPath)
				results, resultErr := os.ReadFile(resultPath)
				if debitErr != nil || resultErr != nil || bytes.Equal(quotaBefore, debits) {
					t.Fatal("original committed issuer histories", debitErr, resultErr)
				}
				if err := prefix.Issue(t.Context(), 2, nil); err != nil {
					t.Fatal("same-Prefix retry of pending issuance", err)
				}
				status := holder.Status()
				if status.Pending || status.Busy || status.Remaining != after.Remaining {
					t.Fatal("retry failed to deposit or charged the holder again", status)
				}
				debitsAfter, debitErr := os.ReadFile(quotaPath)
				resultsAfter, resultErr := os.ReadFile(resultPath)
				if debitErr != nil || resultErr != nil || !bytes.Equal(debits, debitsAfter) || !bytes.Equal(results, resultsAfter) {
					t.Fatal("same-Prefix retry changed issuer histories", debitErr, resultErr)
				}
				if err := errors.Join(prefix.Close(), holder.Close(), clientAuthority.Close()); err != nil {
					t.Fatal("retried holder join", err)
				}
				finishRouteJoinPeers(t, peers, budgets)
				return
			}
			probe, err := net.DialTimeout("tcp", spec.Nodes[0].Endpoint, time.Second)
			if err != nil {
				t.Fatal("issuer stopped before borrower cleanup", err)
			}
			if err := probe.Close(); err != nil {
				t.Fatal("issuer probe", err)
			}
			finishRouteJoinPeers(t, peers, budgets)
			debits, err := os.ReadFile(quotaPath)
			if err != nil || bytes.Equal(quotaBefore, debits) {
				t.Fatal("lost result did not retain issuer debit", err)
			}
			results, err := os.ReadFile(resultPath)
			if err != nil {
				t.Fatal("retained issuer result", err)
			}
			batch, response, ok := committedIssuerPair(resultBefore, results)
			if !ok {
				t.Fatal("committed request/result unavailable")
			}
			defer clear(batch)
			defer clear(response)
			originalRequest, requestErr := admission.DecodePermissionRequest(request)
			issuedRequest, issueErr := admission.DecodeClosedTokenBatch(batch)
			if requestErr != nil || issueErr != nil || originalRequest.Permission.HolderKey != issuedRequest.Permission.HolderKey {
				t.Fatal("committed issuance changed holder identity", requestErr, issueErr)
			}
			retry := admissionLocalCommand(t, binary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch, "kind": quota.Admitted})
			if retry.Outcome != "already-issued" || !bytes.Equal(retry.Response, response) {
				t.Fatal("issuer retry changed committed result", retry.Outcome)
			}
			debitsAfter, debitErr := os.ReadFile(quotaPath)
			resultsAfter, resultErr := os.ReadFile(resultPath)
			if debitErr != nil || resultErr != nil || !bytes.Equal(debits, debitsAfter) || !bytes.Equal(results, resultsAfter) {
				t.Fatal("retry changed issuer histories", debitErr, resultErr)
			}
			reopenedAuthority, err := funding.OpenAdmissionAuthority("", &networkPlan)
			if err != nil {
				t.Fatal("reopened holder authority", err)
			}
			defer reopenedAuthority.Close()
			reopened, err := stock.Open(holderRoot, admission.AllocationUser, reopenedAuthority.ObserveFacts)
			if err != nil {
				t.Fatal("reopened holder", err)
			}
			defer reopened.Close()
			if status := reopened.Status(); status.Accepted || status.Pending || status.Busy || status.BootstrapRemaining != 0 {
				t.Fatal("reopen reconstructed volatile Stock", status)
			}
			for _, payload := range [][]byte{permission, response} {
				if err := reopened.Import(digest, payload); err == nil {
					t.Fatal("reopened holder adopted old material")
				}
			}
			fresh, freshDigest, err := reopened.Request([3]uint32{2, 8, 0})
			if err != nil {
				t.Fatal("fresh holder request", err)
			}
			freshRequest, err := admission.DecodePermissionRequest(fresh)
			clear(fresh)
			if err != nil || freshDigest == digest || freshRequest.Permission.HolderKey == originalRequest.Permission.HolderKey {
				t.Fatal("reopened holder reconstructed identity", err)
			}
			if err := reopened.Import(freshDigest, permission); err == nil {
				t.Fatal("fresh request adopted old permission")
			}
			if err := errors.Join(reopened.Close(), reopenedAuthority.Close()); err != nil {
				t.Fatal("reopened holder join", err)
			}
			return
		}
		if postCommitObservations.Load() != 2 || !errors.Is(issueErr, context.Canceled) {
			t.Fatal("verified issuance lost caller cancellation", postCommitObservations.Load(), issueErr)
		}
		quotaAfter, err := os.ReadFile(quotaPath)
		if err != nil || bytes.Equal(quotaBefore, quotaAfter) {
			t.Fatal("issuer did not commit before cancellation", err)
		}
		after := holder.Status()
		if after.Busy || after.Pending || after.Remaining[1] != before.Remaining[1]-2 || after.BootstrapRemaining != 0 {
			t.Fatal("retired verified completion changed allocation", before, after)
		}
		presentationMu.Lock()
		probe := forwarding
		presentationMu.Unlock()
		if probe.RecipientNodeID == [32]byte{} {
			t.Fatal("missing genuine Forwarding presentation")
		}
		probe.ChannelNonce = [32]byte{97}
		raw, probeErr := holder.Take(t.Context(), probe, 2)
		clear(raw)
		if stock.TransferFailureStage(probeErr) != "stock" {
			t.Fatal("retired verified tokens became available or probe failed elsewhere", probeErr)
		}
		if err := errors.Join(prefix.Close(), holder.Close(), clientAuthority.Close()); err != nil {
			t.Fatal("joined holder", err)
		}
		finishRouteJoinPeers(t, peers, budgets)
		return
	}
	if draftClockLoss {
		clientAuthority, err := funding.OpenAdmissionAuthority("", &networkPlan)
		if err != nil {
			t.Fatal("holder authority", err)
		}
		defer clientAuthority.Close()
		var armed, triggered atomic.Bool
		observe := func() (admission.AuthorityFacts, time.Time, error) {
			if armed.CompareAndSwap(true, false) {
				if err := os.Remove(holderClock); err != nil {
					return admission.AuthorityFacts{}, time.Time{}, err
				}
				triggered.Store(true)
			}
			return clientAuthority.ObserveFacts()
		}
		holder, err := stock.Open(processfixture.TempDir(t), admission.AllocationUser, observe)
		if err != nil {
			t.Fatal("holder Stock", err)
		}
		defer holder.Close()
		request, digest, err := holder.Request([3]uint32{2, 4, 0})
		if err != nil {
			t.Fatal("holder request", err)
		}
		if err := holder.Import(digest, processfixture.FixturePermission(t, request, facts, spec.Authority, now)); err != nil {
			t.Fatal("holder permission", err)
		}
		plan := funding.Plan{EntryRoot: route["entry_root"].(string), InteriorRoot: route["interior_root"].(string), HostingRoot: budget,
			Domain: 1, Deadline: route["deadline"].(time.Time), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		boot, err := session.OpenBootstrap(t.Context(), plan, funding.Observe(clientAuthority.Current, clientAuthority.Witness, clientAuthority.Presentation), holder)
		if err != nil {
			t.Fatal("genuine Bootstrap", err)
		}
		defer boot.Close()
		before := holder.Status()
		quotaPath := filepath.Join(issuerPlan.AdmissionRoot, "admission.journal")
		quotaBefore, err := os.ReadFile(quotaPath)
		if err != nil {
			t.Fatal("issuer quota journal", err)
		}
		armed.Store(true)
		issueErr := boot.Issue(t.Context(), 2, [][32]byte{{12}})
		if !triggered.Load() || issueErr == nil {
			t.Fatal("draft bypassed fresh Stock authority", triggered.Load(), issueErr)
		}
		after := holder.Status()
		if after.Pending || after.Busy || after.Remaining != before.Remaining || after.BootstrapRemaining != before.BootstrapRemaining {
			t.Fatal("clock loss reserved a batch or changed allowance", before, after)
		}
		quotaAfter, err := os.ReadFile(quotaPath)
		if err != nil || !bytes.Equal(quotaBefore, quotaAfter) {
			t.Fatal("refused draft changed issuer quota", err)
		}
		for i, root := range spendRoots {
			current, err := os.ReadFile(filepath.Join(root, "closed-token-spends"))
			if err != nil || !bytes.Equal(current, originalSpends[i]) {
				t.Fatal("clock refusal changed receiving spend", i, err)
			}
		}
		if err := os.WriteFile(holderClock, nil, 0600); err != nil {
			t.Fatal("restore clock", err)
		}
		if _, _, err := clientAuthority.ObserveFacts(); err != nil {
			t.Fatal("restored holder authority", err)
		}
		closeErr := boot.Close()
		if closeErr != nil && boot.Close() != closeErr {
			t.Fatal("Bootstrap changed its terminal result", closeErr)
		}
		if err := errors.Join(holder.Close(), clientAuthority.Close()); err != nil {
			t.Fatal("holder joined result", err)
		}
		finishRouteJoinPeers(t, peers, budgets)
		return
	}
	send, closeHolder := admissionLocalConsoleResult(t, binary, "holder", map[string]any{"root": processfixture.TempDir(t), "network": networkPlan, "role": admission.AllocationUser, "route": route})
	maxima := [3]uint32{2, 4, 0}
	if sequential {
		maxima = [3]uint32{4, 8, 0}
	}
	request := send(holderCommand{Operation: "request", Maxima: maxima})
	if request.Outcome != "completed" {
		t.Fatal("holder request", request.Outcome)
	}
	permission := processfixture.FixturePermission(t, request.Request, facts, spec.Authority, now)
	if reply := send(holderCommand{Operation: "import", Digest: request.Digest, Payload: permission}); reply.Outcome != "completed" {
		t.Fatal("holder import", reply.Outcome)
	}
	assertStock := func(remaining [3]uint32, bootstrap uint8) {
		t.Helper()
		reply := send(holderCommand{Operation: "status"})
		if reply.Outcome != "completed" || !reply.Stock.Accepted || reply.Stock.Pending || reply.Stock.Busy || reply.Stock.Closed ||
			reply.Stock.Remaining != remaining || reply.Stock.BootstrapRemaining != bootstrap {
			t.Fatal("holder stock differs", reply.Outcome, reply.Stock)
		}
	}
	issuerBudget := budgets[len(spendRoots)-1]
	assertPeerReserves := func(want int) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			var roles [2]int
			for index, root := range budgets[:len(spendRoots)-1] {
				reserved, err := routeBudgetReserved(root)
				if err != nil {
					t.Fatal("observe receiving Hosting", err)
				}
				if reserved != 0 {
					roles[index/2]++
				}
			}
			active := roles[0] + roles[1]
			if roles[0] > 1 || roles[1] > 1 {
				t.Fatalf("bootstrap retained multiple reservations for one role: %v", roles)
			}
			if active == want {
				reserved, err := routeBudgetReserved(issuerBudget)
				if err != nil || reserved != 0 {
					t.Fatalf("Issuer retained %d Hosting bytes: %v", reserved, err)
				}
				return
			}
			// A live Prefix needs both owners already reserved. A closed
			// Prefix permits its remote parents to finish terminal I/O.
			if want != 0 {
				t.Fatalf("receiving reservations: got %d live roots, want %d", active, want)
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				t.Fatalf("receiving reservations did not return: %v: %v", roles, ctx.Err())
			}
		}
	}
	waitIssuerReturn := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			reserved, err := routeBudgetReserved(issuerBudget)
			if err != nil {
				t.Fatal("observe Issuer Hosting", err)
			}
			if reserved == 0 {
				return
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				t.Fatalf("Issuer retained %d Hosting bytes: %v", reserved, ctx.Err())
			}
		}
	}
	assertStock(maxima, 2)
	var failedBootstrapClose bool
	if loseHolderClock {
		if reply := send(holderCommand{Operation: "bootstrap-open"}); reply.Outcome != "completed" {
			t.Fatal("bootstrap open", reply.Outcome)
		}
		if err := os.Remove(holderClock); err != nil {
			t.Fatal(err)
		}
		if reply := send(holderCommand{Operation: "issuer-issue", Class: 2}); reply.Outcome == "completed" {
			t.Fatal("issuer accepted unavailable clock")
		}
		if err := os.WriteFile(holderClock, nil, 0600); err != nil {
			t.Fatal(err)
		}
		assertStock([3]uint32{2, 4, 0}, 2)
		for i, root := range spendRoots {
			current, err := os.ReadFile(filepath.Join(root, "closed-token-spends"))
			if err != nil || !bytes.Equal(current, originalSpends[i]) {
				t.Fatal("refused issuance changed receiving Admission", i, err)
			}
		}
		// The original Bootstrap is retired, regardless of whether the holder's
		// watcher or the explicit close observed the failed authority first.
		closed := send(holderCommand{Operation: "bootstrap-close"})
		if closed.Outcome != "completed" && closed.Outcome != "refused" {
			t.Fatal("unexpected Bootstrap close", closed.Outcome)
		}
		failedBootstrapClose = closed.Outcome == "refused"
	} else {
		if reply := send(holderCommand{Operation: "bootstrap-open"}); reply.Outcome != "completed" {
			t.Fatal("bootstrap open", reply.Outcome)
		}
		waitIssuerReturn()
		assertPeerReserves(2)
		commands := []holderCommand{{Operation: "issuer-issue", Class: 2}, {Operation: "issuer-issue", Class: 1}}
		if sequential {
			commands[1].Receivers = [][32]byte{issuerID, issuerID, issuerID}
		}
		if singleBootstrap {
			commands = commands[:1]
		}
		for _, command := range commands {
			if reply := send(command); reply.Outcome != "completed" {
				t.Fatal("bootstrap", command.Operation, command.Class, reply.Outcome)
			}
		}
		if singleBootstrap {
			assertStock([3]uint32{2, 2, 0}, 1)
		} else if sequential {
			assertStock([3]uint32{1, 6, 0}, 0)
			if reply := send(holderCommand{Operation: "issuer-issue", Class: 1}); reply.Outcome == "completed" {
				t.Fatal("third bootstrap batch accepted")
			}
			assertStock([3]uint32{1, 6, 0}, 0)
		} else {
			assertStock([3]uint32{1, 2, 0}, 0)
			if reply := send(holderCommand{Operation: "issuer-issue", Class: 1}); reply.Outcome == "completed" {
				t.Fatal("third bootstrap batch accepted")
			}
			assertStock([3]uint32{1, 2, 0}, 0)
		}
		for i, root := range spendRoots {
			current, err := os.ReadFile(filepath.Join(root, "closed-token-spends"))
			if err != nil || !bytes.Equal(current, originalSpends[i]) {
				t.Fatal("bootstrap consumed receiving Admission", i, err)
			}
		}
		// The issuer's temporary reservation must have returned while the two
		// original forwarding parents remain live.
		waitIssuerReturn()
		assertPeerReserves(2)
		if reply := send(holderCommand{Operation: "bootstrap-close"}); reply.Outcome != "completed" {
			t.Fatal("bootstrap close", reply.Outcome)
		}
		assertPeerReserves(0)
		if reply := send(holderCommand{Operation: "issuer-issue", Class: 1}); reply.Outcome == "completed" {
			t.Fatal("retired bootstrap reopened an issuer exchange")
		}
		if singleBootstrap {
			assertStock([3]uint32{2, 2, 0}, 1)
		} else {
			if sequential {
				assertStock([3]uint32{1, 6, 0}, 0)
			} else {
				assertStock([3]uint32{1, 2, 0}, 0)
			}
			commands := []holderCommand{{Operation: "prefix-open"}, {Operation: "issuer-issue", Class: 2}, {Operation: "prefix-close"}}
			if sequential {
				commands = []holderCommand{{Operation: "prefix-open"}, {Operation: "issuer-issue", Class: 1}, {Operation: "issuer-issue", Class: 2}, {Operation: "prefix-close"}}
			}
			issuerSpend := filepath.Join(spendRoots[len(spendRoots)-1], "closed-token-spends")
			lastIssuerSpend := originalSpends[len(originalSpends)-1]
			for _, command := range commands {
				if reply := send(command); reply.Outcome != "completed" {
					t.Fatal("ordinary issuer", command.Operation, command.Class, reply.Outcome)
				}
				if command.Operation == "issuer-issue" {
					current, err := os.ReadFile(issuerSpend)
					if err != nil || bytes.Equal(current, lastIssuerSpend) {
						t.Fatal("ordinary exchange did not spend fresh Issuer Control", command.Class, err)
					}
					lastIssuerSpend = current
					waitIssuerTerminationReturn(t, issuerBudget, receiverTermination.Tx+receiverTermination.Rx)
					assertPeerReserves(2)
				}
			}
			if sequential {
				assertStock([3]uint32{0, 4, 0}, 0)
			}
			changed := 0
			for i, root := range spendRoots {
				current, err := os.ReadFile(filepath.Join(root, "closed-token-spends"))
				if err != nil {
					t.Fatal("ordinary issuer spending journal", i, err)
				}
				if !bytes.Equal(current, originalSpends[i]) {
					changed++
				} else if i == len(spendRoots)-1 {
					t.Fatal("ordinary issuer did not spend its Control right")
				}
			}
			if changed != 3 {
				t.Fatalf("ordinary issuer changed %d spending journals, want Entry, Interior and Issuer", changed)
			}
			// Spend the ordinary batch on a second Prefix, then prove that the same
			// retained holder cannot reopen after its selected Forward rights are spent.
			for _, operation := range []string{"prefix-open", "prefix-close"} {
				if reply := send(holderCommand{Operation: operation}); reply.Outcome != "completed" {
					t.Fatal("ordinary funded prefix", operation, reply.Outcome)
				}
			}
			if reply := send(holderCommand{Operation: "prefix-open"}); reply.Outcome == "completed" {
				t.Fatal("holder reopened Prefix using spent Forward rights")
			}
		}
	}
	holderErr := closeHolder()
	if failedBootstrapClose {
		var exit *exec.ExitError
		if !errors.As(holderErr, &exit) || exit.ExitCode() != 1 {
			t.Fatal("failed Bootstrap close lost terminal result", holderErr)
		}
	} else if holderErr != nil {
		t.Fatal("holder terminal", holderErr)
	}
	for _, peer := range peers {
		if err := processfixture.Interrupt(peer.command); err != nil {
			t.Fatal("interrupt receiver", err)
		}
	}
	for _, peer := range peers {
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
		observed, observeErr := owner.Observe(t.Context())
		closeErr := owner.Close()
		if observeErr != nil || closeErr != nil || observed.ReservedBytes != 0 {
			t.Fatal("joined receiver retained Hosting", observeErr, closeErr, observed.ReservedBytes)
		}
	}
}

func waitIssuerTerminationReturn(t *testing.T, budgetRoot string, terminationBytes uint64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		reserved, err := routeBudgetReserved(budgetRoot)
		if err != nil || reserved > terminationBytes {
			t.Fatalf("Issuer returned with Hosting work still held: reserved=%d termination=%d error=%v", reserved, terminationBytes, err)
		}
		if reserved == 0 {
			return
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatalf("Issuer termination share did not return: reserved=%d error=%v", reserved, ctx.Err())
		}
	}
}

// committedIssuerPair independently checks both appended hash-chain frames.
// It returns no partial record while an Issuer is still writing its result.
// committedQuotaDebit checks one complete committed hash-chained record.
func committedQuotaDebit(before, after []byte) bool {
	const size = 193
	if len(before) < 33 || len(after) != len(before)+size || !bytes.Equal(before, after[:len(before)]) || before[len(before)-1] != 1 {
		return false
	}
	record := after[len(before):]
	if record[len(record)-1] != 1 {
		return false
	}
	hash := sha256.New()
	_, _ = hash.Write(before[len(before)-33 : len(before)-1])
	_, _ = hash.Write(record[:len(record)-33])
	return bytes.Equal(hash.Sum(nil), record[len(record)-33:len(record)-1])
}

func committedIssuerPair(before, after []byte) ([]byte, []byte, bool) {
	if len(before) < 32 || len(after) <= len(before) || !bytes.HasPrefix(after, before) {
		return nil, nil, false
	}
	previous := bytes.Clone(before[len(before)-32:])
	rest := after[len(before):]
	read := func() ([]byte, bool) {
		if len(rest) < 36 {
			return nil, false
		}
		n := int(binary.BigEndian.Uint32(rest[:4]))
		if n < 1 || n > 16348 || len(rest) < n+36 {
			return nil, false
		}
		body := rest[4 : 4+n]
		h := sha256.New()
		_, _ = h.Write(previous)
		_, _ = h.Write(body)
		digest := h.Sum(nil)
		if !bytes.Equal(digest, rest[4+n:n+36]) {
			return nil, false
		}
		previous = digest
		rest = rest[n+36:]
		return body, true
	}
	first, ok := read()
	if !ok || len(first) <= 9 || first[0] != byte(quota.Admitted) {
		return nil, nil, false
	}
	second, ok := read()
	if !ok || len(rest) != 0 {
		return nil, nil, false
	}
	return bytes.Clone(first[9:]), bytes.Clone(second), true
}
