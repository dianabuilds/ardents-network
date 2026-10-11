//go:build linux && text_worker_installed

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The actual production consumer runs inside the original installed Endpoint
// MainPID. All remote fixtures use genuine new signed owners and Carriers;
// neither this test nor its result supplies Installation or Service readiness.
func TestInstalledExecutionRouteBothCarriers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
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
	x := newRouteIssuerFixture(t, carrier, func(ctx context.Context, bootstrap bool, _, _ []byte) error {
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
	}, nil, nil, nil)
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
	clock, closeClock := joinProcessClock(t)
	var clockOnce sync.Once
	joinClock := func() { clockOnce.Do(closeClock) }
	defer joinClock()
	var key [32]byte
	copy(key[:], f.spec.Authority.Public().(ed25519.PublicKey))
	network := networkAuthorityPlan{Root: filepath.Join(t.TempDir(), "network"), NetworkID: f.spec.NetworkID,
		Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: clock}
	config, err := networkStateConfig(&network)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = seed.Accept(ctx, bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials)
	if err == nil {
		_, err = seed.AcceptClosedProfile(bundle.Profile)
	}
	if err := errors.Join(err, seed.Close()); err != nil {
		t.Fatal("independent signed holder State", err)
	}
	local := t.TempDir()
	budgetRoot := routeProcessBudget(t)
	plan := routePrefixPlan{EntryRoot: filepath.Join(local, "entry"), InteriorRoot: filepath.Join(local, "interior"),
		HostingRoot: budgetRoot, Domain: 1, Deadline: time.Now().Add(35 * time.Second).UTC().Truncate(time.Second),
		Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	path := hostingConfig(t, map[string]any{"generation": [32]byte{21}, "principal": [32]byte{22},
		"holder": holderPlan{Root: t.TempDir(), Network: &network, Role: admission.AllocationUser, Route: &plan}})
	input, sendInput := io.Pipe()
	readOutput, output := io.Pipe()
	var diagnostics bytes.Buffer
	finished := make(chan int, 1)
	go func() {
		defer input.Close()
		defer output.Close()
		finished <- runExecutionRouteHolder(caller, []string{"--config", path}, input, output, &diagnostics)
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
		if err := readConsoleReply(replies, &reply); err != nil {
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
				result.err = readConsoleReply(replies, &result.reply)
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
		stopInstalledExecutionWorker(t, ctx, "reader")
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
	waitRouteIssuerReceivingBorrowers(t, ctx, x)
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
			// The actual adapter's exact peer-retirement classification is
			// diagnostic, never a clean result. Every other leaf still fails.
			if !retirement || !transport.IsPeerRetirementCause(joinedErr) {
				t.Fatal("joined receiver", joinedErr)
			}
			t.Log("retained failed receiver after deliberate retirement", joinedErr)
		}
	}
	// Reopen the genuine independent receiving histories after all physical
	// borrowers joined. Retirement and late signed RESULT cannot refund spend.
	var replayed int32
	for index, spent := range x.spends {
		if err := spent.owner.Close(); err != nil {
			t.Fatal("joined original spend owner", err)
		}
		reopened, err := receiving.Open(spent.root, spent.binding, spent.observe)
		if err != nil {
			t.Fatal("retained spend reopen", err)
		}
		t.Cleanup(func() { _ = reopened.Close() })
		spent.mu.Lock()
		tokens := append(spent.tokens[:0:0], spent.tokens...)
		spent.mu.Unlock()
		for _, token := range tokens {
			current, err := spent.observe()
			if err != nil {
				t.Fatal("replay current signed authority", err)
			}
			if _, err := receiving.VerifyToken(current.Profile, spent.binding, token.class, token.raw, current.Now); err != nil {
				t.Fatal("replay must still be a valid signed token", err)
			}
			reserved, returned := 0, 0
			grant, replayErr := reopened.Accept(ctx, token.class, token.raw, current.Now.Add(5*time.Second), func() (func() error, error) {
				reserved++
				release, err := networkTestReservation(t, x.budgets[index], current.Now.Add(5*time.Second))
				if err != nil {
					return nil, err
				}
				return func() error { returned++; return release() }, nil
			})
			if releaseErr := grant.Release(); releaseErr != nil {
				t.Fatal("replay grant cleanup", releaseErr)
			}
			if replayErr == nil || !strings.Contains(replayErr.Error(), "closed token is already spent") || reserved != 1 || returned != 1 {
				t.Fatalf("reopened replay = %v, reservation %d/%d", replayErr, reserved, returned)
			}
			replayed++
		}
		if err := reopened.Close(); err != nil {
			t.Fatal("joined reopened spend owner", err)
		}
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

// This independent fixture actor stops the sole observed original reader while
// its production holder retains a spent, open Prefix. No replacement or second
// launch runs in this fixture; the runtime owns interruption and physical join.
func stopInstalledExecutionWorker(t *testing.T, ctx context.Context, role string) {
	t.Helper()
	if role != "reader" && role != "publisher" {
		t.Fatal("invalid environment: unknown fixed worker role")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := func(args ...string) string {
		t.Helper()
		child := exec.CommandContext(bounded, "/usr/bin/systemctl", args...)
		child.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
		body, err := child.Output()
		if err != nil {
			t.Fatal("independent original worker observation/stop", err)
		}
		return strings.TrimSpace(string(body))
	}
	listing := command("--system", "--no-pager", "--plain", "--no-legend", "list-units", "--state=active", "ardents-text-"+role+"@*.service")
	rows := strings.Split(listing, "\n")
	if len(rows) != 1 {
		t.Fatal("loss fixture requires exactly one active original worker")
	}
	fields := strings.Fields(rows[0])
	if len(fields) == 0 || !regexp.MustCompile(`^ardents-text-`+role+`@[0-9]+-[1-9][0-9]*-[1-9][0-9]*\.service$`).MatchString(fields[0]) {
		t.Fatal("loss fixture worker identity is unavailable")
	}
	unit := fields[0]
	observe := func() string {
		return command("--system", "show", unit, "-p", "InvocationID", "-p", "MainPID", "-p", "ControlGroup")
	}
	original := observe()
	facts := make(map[string]string)
	for _, row := range strings.Split(original, "\n") {
		name, value, ok := strings.Cut(row, "=")
		if !ok || facts[name] != "" {
			t.Fatal("loss fixture original worker observation is malformed")
		}
		facts[name] = value
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(facts["InvocationID"]) ||
		!regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(facts["MainPID"]) ||
		facts["ControlGroup"] != "/system.slice/"+unit || original != observe() {
		t.Fatal("loss fixture original worker changed before stop")
	}
	command("--system", "--no-ask-password", "--no-pager", "stop", unit)
}
