//go:build linux && document_worker_installed

package endpoint_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/session"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// The actual production consumer runs inside the original installed Endpoint
// MainPID. All remote fixtures use genuine new signed owners and Carriers;
// neither this test nor its result supplies Installation or Service readiness.
func TestInstalledExecutionRouteTCP(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			t.Run("service-authentication", func(t *testing.T) { runInstalledConnectionAuthentication(t, carrier) })
			t.Run("service-text", func(t *testing.T) { runInstalledConnectionText(t, carrier, false) })
			t.Run("service-setup-handoff", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "setup-handoff") })
			t.Run("service-recovery", func(t *testing.T) { runInstalledConnectionText(t, carrier, true) })
			t.Run("publication", func(t *testing.T) { runInstalledPublisher(t, carrier) })
			t.Run("publication-ack", func(t *testing.T) { runInstalledPublisherACK(t, carrier, "") })
			t.Run("publication-withdraw-ack", func(t *testing.T) { runInstalledPublisherACK(t, carrier, "withdraw") })
			t.Run("publication-caller-ack", func(t *testing.T) { runInstalledPublisherACK(t, carrier, "caller") })
			t.Run("publication-worker-ack", func(t *testing.T) { runInstalledPublisherACK(t, carrier, "worker") })
			t.Run("publication-network-ack", func(t *testing.T) { runInstalledPublisherACK(t, carrier, "network") })
			t.Run("publication-decoded-network-ack", func(t *testing.T) { runInstalledPublisherACK(t, carrier, "decoded-network") })
			t.Run("publication-delivery-close", func(t *testing.T) { runInstalledPublisherACK(t, carrier, "delivery-close") })
			for _, mode := range []string{"joined", "worker-loss", "worker-loss-io", "clock-loss-io", "caller-loss-io"} {
				t.Run(mode, func(t *testing.T) { runInstalledExecutionRoute(t, carrier, mode) })
			}
		})
	}
}

func runInstalledExecutionRoute(t *testing.T, carrier transport.CarrierProfile, mode string) {
	workerLoss := mode == "worker-loss" || mode == "worker-loss-io"
	lossDuringIO := mode == "worker-loss-io" || mode == "clock-loss-io" || mode == "caller-loss-io"
	retirement := mode != "joined"
	issued, release := make(chan struct{}, 1), make(chan struct{})
	var releaseOnce sync.Once
	releaseResult := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseResult()
	x := newInstalledRouteIssuerFixture(t, carrier, func(ctx context.Context, bootstrap bool) error {
		if bootstrap || !lossDuringIO {
			return nil
		}
		// This hook follows genuine irreversible issuing and verified signed
		// output. It delays RESULT; it supplies neither tokens nor authority.
		select {
		case issued <- struct{}{}:
		default:
			return errors.New("I/O loss fixture observed another ordinary issuance")
		}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	// The independent fixture may wait for a usable real permission hour.
	// Start the consumer bound after that pre-operation preparation, as
	// other maintained Route scenarios do; no runtime deadline is reset.
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	caller, cancelCaller := context.WithCancel(ctx)
	defer cancelCaller()
	f := x.network
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := networkinputs.StartObservedClock(t)
	var clockOnce sync.Once
	joinClock := func() { clockOnce.Do(closeClock) }
	defer joinClock()
	var key [32]byte
	copy(key[:], f.spec.Authority.Public().(ed25519.PublicKey))
	network := state.AuthorityDeclaration{Root: filepath.Join(t.TempDir(), "network"), NetworkID: f.spec.NetworkID,
		Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: clock}
	config, err := network.Config()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = seed.AcceptEpoch(ctx, bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials)
	if err == nil {
		_, err = seed.AcceptClosedProfile(bundle.Profile)
	}
	if err := errors.Join(err, seed.Close()); err != nil {
		t.Fatal("independent signed holder State", err)
	}
	local := t.TempDir()
	budgetRoot := processfixture.ProcessBudget(t)
	plan := funding.Declaration{EntryRoot: filepath.Join(local, "entry"), InteriorRoot: filepath.Join(local, "interior"),
		HostingRoot: budgetRoot, Domain: 1, Deadline: time.Now().Add(35 * time.Second).UTC().Truncate(time.Second),
		Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	route := plan.Plan()
	executionPlan := session.ExecutionHolderPlan{Generation: [32]byte{21}, Principal: [32]byte{22},
		Holder: session.LocalHolderPlan{Root: t.TempDir(), Network: &network, Role: admission.AllocationUser,
			Options: session.Config{Route: &route}}}
	input, sendInput := io.Pipe()
	readOutput, output := io.Pipe()
	var diagnostics bytes.Buffer
	finished := make(chan int, 1)
	go func() {
		defer input.Close()
		defer output.Close()
		finished <- session.RunExecutionHolder(caller, executionPlan, input, output, &diagnostics, true)
	}()
	waited := false
	type commandResult struct {
		reply localAdmissionReply
		err   error
	}
	var pending chan commandResult
	t.Cleanup(func() {
		cancelCaller()
		cancel()
		_ = sendInput.Close()
		_ = readOutput.Close()
		if !waited {
			<-finished
		}
		if pending != nil {
			<-pending
		}
		if t.Failed() {
			t.Log("joined installed holder diagnostics", diagnostics.String())
		}
	})
	encoder, replies := json.NewEncoder(sendInput), bufio.NewReader(readOutput)
	send := func(command any) localAdmissionReply {
		t.Helper()
		named, ok := command.(holderCommand)
		if !ok {
			t.Fatalf("installed holder unexpected command %T", command)
		}
		if err := encoder.Encode(command); err != nil {
			t.Fatalf("installed holder %s input: %v", named.Operation, err)
		}
		var reply localAdmissionReply
		if err := processfixture.ReadConsoleReply(replies, &reply); err != nil {
			t.Fatalf("installed holder %s output: %v", named.Operation, err)
		}
		return reply
	}
	routeConsolePermissionStock(t, f, send, [3]uint32{2, 4, 0})
	commands := []holderCommand{{Operation: "bootstrap-open"}, {Operation: "issuer-issue", Class: 2},
		{Operation: "issuer-issue", Class: 1}, {Operation: "bootstrap-close"}, {Operation: "prefix-open"},
		{Operation: "issuer-issue", Class: 2}, {Operation: "prefix-close"}, {Operation: "close"}}
	if retirement {
		commands = commands[:5]
	}
	for _, command := range commands {
		if reply := send(command); reply.Outcome != "completed" {
			t.Fatalf("installed holder %s refused", command.Operation)
		}
	}
	if lossDuringIO {
		pending = make(chan commandResult, 1)
		go func() {
			var result commandResult
			result.err = encoder.Encode(holderCommand{Operation: "issuer-issue", Class: 2})
			if result.err == nil {
				result.err = processfixture.ReadConsoleReply(replies, &result.reply)
			}
			pending <- result
		}()
		select {
		case <-issued:
		case <-ctx.Done():
			t.Fatal("ordinary admitted issuer did not reach genuine signed RESULT I/O")
		}
	}
	if workerLoss {
		endpointfixture.StopInstalledExecutionWorker(t, ctx, "reader")
	}
	if mode == "clock-loss-io" {
		// Retire the real observation updater before removing its file. The
		// holder's genuine State remains independently owned and must refuse
		// current signed authority at RESULT; no supplied flag revokes it.
		joinClock()
		if err := os.Rename(clock, clock+".retired"); err != nil {
			t.Fatal("original clock observation retirement", err)
		}
		if caller.Err() != nil {
			t.Fatal("caller ended before genuine holder State loss")
		}
	}
	if mode == "caller-loss-io" {
		cancelCaller()
	}
	if lossDuringIO {
		releaseResult()
	}
	if mode == "clock-loss-io" {
		select {
		case result := <-pending:
			pending = nil
			if result.err != nil || result.reply.Outcome != "refused" || caller.Err() != nil {
				t.Fatalf("genuine State loss did not refuse signed RESULT before caller retirement: %v %+v", result.err, result.reply)
			}
		case <-ctx.Done():
			t.Fatal("holder did not report genuine State loss at signed RESULT")
		}
		// Network refusal is distinct from local worker loss. The fixture
		// caller now retires its otherwise live console and original worker.
		cancelCaller()
	}
	if !retirement {
		_ = sendInput.Close()
	}
	select {
	case code := <-finished:
		waited = true
		if (!retirement && code != 0) || (retirement && code != 1) {
			t.Fatalf("installed holder exit %d: %s", code, diagnostics.String())
		}
	case <-ctx.Done():
		t.Fatal("installed holder did not join original operation/worker")
	}
	if pending != nil {
		result := <-pending
		pending = nil
		if result.err == nil && result.reply.Outcome == "completed" {
			t.Fatal("late genuine signed RESULT completed after original operation retirement")
		}
	}
	if mode == "clock-loss-io" {
		// Reopen the original signed history after its real consumer joined;
		// require the specific clock refusal, not an unrelated failed fixture.
		observed, err := state.Open(config)
		if err != nil {
			t.Fatal("original signed State reopen", err)
		}
		_, unavailable := observed.CurrentRuntime()
		closeErr := observed.Close()
		if unavailable == nil || !strings.Contains(unavailable.Error(), "clock confidence is outside the two-second bound") || closeErr != nil {
			t.Fatalf("original State loss cause changed: %v; close=%v", unavailable, closeErr)
		}
	}
	ordinaryIssues, admissions := int32(1), int32(3)
	if workerLoss && !lossDuringIO {
		ordinaryIssues, admissions = 0, 2
	}
	if x.bootstrapIssues.Load() != 2 || x.ordinaryIssues.Load() != ordinaryIssues || x.admissions.Load() != admissions {
		t.Fatal("installed holder bypassed actual bootstrap, spend or issuing owners")
	}
	processfixture.WaitReturnedReservations(t, ctx, x.budgets, &x.bootstrapHolds, &x.bootstrapReturns)
	for _, server := range x.servers {
		joinedErr := server.Close()
		select {
		case <-server.Done():
		default:
			t.Fatal("receiver Close returned before original listener join")
		}
		if repeated := server.Close(); repeated != joinedErr {
			t.Fatal("receiver lost its first joined terminal result")
		}
		if joinedErr != nil {
			// Deliberate retirement can interrupt an already started frame.
			// Receiver.Close also interrupts its own socket while a delayed
			// RESULT is unwinding. Retain that local closed-only failure in the
			// I/O-loss cases; it is not a peer witness or a clean result. Both
			// classifiers inspect every leaf, so unrelated failures still fail.
			localInterrupted := lossDuringIO && routeExpiryCloseOnly(joinedErr)
			if !retirement || !(transport.IsPeerRetirementCause(joinedErr) || localInterrupted) {
				t.Fatal("joined receiver", joinedErr)
			}
			t.Log("retained failed receiver after deliberate retirement", joinedErr)
		}
	}
	// Reopen the genuine independent receiving histories after all physical
	// borrowers joined. Retirement and late signed RESULT cannot refund spend.
	var replayed int32
	for index, spent := range x.spends {
		replayed += spent.RequireRetained(t, ctx, func(deadline time.Time) (func() error, error) {
			return networkTestReservation(t, x.budgets[index], deadline)
		})
	}
	if replayed != admissions {
		t.Fatalf("reopened %d genuine spends, expected %d", replayed, admissions)
	}
	if x.bootstrapHolds.Load() != 4 || x.bootstrapReturns.Load() != 4 {
		t.Fatal("bootstrap physical reservations were lost")
	}
	for _, budget := range x.budgets {
		observation, err := budget.Observe(ctx)
		if err != nil || observation.ReservedBytes != 0 {
			t.Fatal("receiver retained unjoined Hosting", err)
		}
	}
	budget, err := hosting.Open(budgetRoot)
	if err != nil {
		t.Fatal("joined holder budget reopen", err)
	}
	observation, err := budget.Observe(ctx)
	if err := errors.Join(err, budget.Close()); err != nil || observation.ReservedBytes != 0 {
		t.Fatal("holder retained unjoined Hosting", err)
	}
}

// routeExpiryCloseOnly rejects joined failures containing any non-close cause.
func routeExpiryCloseOnly(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !routeExpiryCloseOnly(cause) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return routeExpiryCloseOnly(wrapped.Unwrap())
	}
	return errors.Is(err, net.ErrClosed)
}
