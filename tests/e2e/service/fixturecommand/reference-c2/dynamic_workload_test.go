//go:build browsercompat

package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConfiguredDynamicWorkloadUsesOneConnectionAndClosesAfterEveryCycle(t *testing.T) {
	proof := filepath.Join(t.TempDir(), "proof")
	plan := dynamicWorkloadPlan{cycles: 3, interval: 50 * time.Millisecond, cycleDeadline: time.Second}
	client, accept := dynamicWorkloadTestClient(t)
	server := make(chan dynamicWorkloadServerResult, 1)
	go func() {
		connection, err := accept()
		if err != nil {
			server <- dynamicWorkloadServerResult{err: err}
			return
		}
		workload, serveErr := serveConfiguredDynamic(connection, proof, dynamicWorkloadControls{}, plan, "", "")
		server <- dynamicWorkloadServerResult{workload: workload, err: serveErr}
	}()

	started := time.Now()
	workload, err := exerciseConfiguredDynamic(client, "http://reference.ard/", plan, "", "")
	if err != nil {
		t.Fatal(err)
	}
	result := <-server
	if result.err != nil {
		t.Fatal(result.err)
	}
	if workload.ExpectedCycles != 3 || workload.CompletedCycles != 3 || result.workload.CompletedCycles != 3 {
		t.Fatalf("dynamic workload progress = user %+v / server %+v", workload, result.workload)
	}
	if workload.P50CycleLatencyMicros <= 0 || workload.P50CycleLatencyMicros > workload.P95CycleLatencyMicros ||
		workload.P95CycleLatencyMicros > workload.P99CycleLatencyMicros || workload.P99CycleLatencyMicros > workload.MaximumCycleLatencyMicros {
		t.Fatalf("dynamic workload quantiles = %+v", workload)
	}
	if workload.PlannedStartAtUTC.IsZero() || workload.ActualStartAtUTC.IsZero() ||
		workload.PlannedStartAtUTC.Format("Z07:00")[len(workload.PlannedStartAtUTC.Format("Z07:00"))-1:] != "Z" ||
		workload.ActualStartAtUTC.Format("Z07:00")[len(workload.ActualStartAtUTC.Format("Z07:00"))-1:] != "Z" ||
		workload.ActualStartAtUTC.Before(workload.PlannedStartAtUTC) ||
		workload.ActualStartAtUTC.Sub(workload.PlannedStartAtUTC) >= plan.interval {
		t.Fatalf("dynamic workload planned/actual start = %+v", workload)
	}
	minimumElapsed := time.Duration(plan.cycles) * plan.interval
	if elapsed := time.Since(started); elapsed < minimumElapsed || workload.ElapsedMicros < minimumElapsed.Microseconds() ||
		workload.ElapsedMicros >= (minimumElapsed+plan.cycleDeadline).Microseconds() {
		t.Fatalf("three-cycle workload elapsed = wall %s / retained %d", elapsed, workload.ElapsedMicros)
	}
	if contents, err := os.ReadFile(proof); err != nil || string(contents) != "dynamic-http\n" {
		t.Fatalf("dynamic workload proof = %q / %v", contents, err)
	}
}

func TestConfiguredDynamicWorkloadUsesDeclaredStartLagLimit(t *testing.T) {
	proof := filepath.Join(t.TempDir(), "proof")
	plan := dynamicWorkloadPlan{cycles: 2, interval: 10 * time.Millisecond, cycleDeadline: time.Second,
		maximumStartLag: time.Second}
	client, accept := dynamicWorkloadTestClient(t)
	server := make(chan dynamicWorkloadServerResult, 1)
	go func() {
		connection, err := accept()
		if err != nil {
			server <- dynamicWorkloadServerResult{err: err}
			return
		}
		workload, serveErr := serveConfiguredDynamic(connection, proof, dynamicWorkloadControls{}, plan, "", "")
		server <- dynamicWorkloadServerResult{workload: workload, err: serveErr}
	}()

	workload, err := exerciseConfiguredDynamic(client, "http://reference.ard/", plan, "", "")
	if err != nil {
		t.Fatal(err)
	}
	result := <-server
	if result.err != nil || workload.CompletedCycles != 2 || result.workload.CompletedCycles != 2 {
		t.Fatalf("declared-lag workload = user %+v / server %+v / %v", workload, result.workload, result.err)
	}
	if workload.MaximumStartLagMicros < plan.interval.Microseconds() {
		t.Fatalf("compressed workload maximum lag = %dµs, want at least one 10ms cadence", workload.MaximumStartLagMicros)
	}
}

func TestConfiguredDynamicWorkloadAllowsBoundedInitialBrowserAssembly(t *testing.T) {
	proof := filepath.Join(t.TempDir(), "proof")
	plan := dynamicWorkloadPlan{cycles: 1, interval: 10 * time.Millisecond, cycleDeadline: 75 * time.Millisecond,
		initialRequestGrace: 250 * time.Millisecond}
	client, accept := dynamicWorkloadTestClient(t)
	server := make(chan dynamicWorkloadServerResult, 1)
	go func() {
		connection, err := accept()
		if err != nil {
			server <- dynamicWorkloadServerResult{err: err}
			return
		}
		workload, serveErr := serveConfiguredDynamic(connection, proof, dynamicWorkloadControls{}, plan, "", "")
		server <- dynamicWorkloadServerResult{workload: workload, err: serveErr}
	}()

	// The remote Application is connected before the local browser origin is
	// ready. Its initial wait may exceed one cycle, but must remain bounded.
	time.Sleep(125 * time.Millisecond)
	workload, err := exerciseConfiguredDynamic(client, "http://reference.ard/", plan, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result := <-server; result.err != nil || workload.CompletedCycles != 1 || result.workload.CompletedCycles != 1 {
		t.Fatalf("bounded initial browser assembly = user %+v / server %+v / %v", workload, result.workload, result.err)
	}
}

func TestConfiguredDynamicApplicationLossFollowsWarmupWithoutReplay(t *testing.T) {
	proof := filepath.Join(t.TempDir(), "proof")
	plan := dynamicWorkloadPlan{cycles: 2, interval: 50 * time.Millisecond, cycleDeadline: time.Second}
	client, accept := dynamicWorkloadTestClient(t)
	server := make(chan dynamicWorkloadServerResult, 1)
	go func() {
		connection, err := accept()
		if err != nil {
			server <- dynamicWorkloadServerResult{err: err}
			return
		}
		workload, serveErr := serveConfiguredDynamic(connection, proof, dynamicWorkloadControls{}, plan, publisherTerminalApplicationReset, "")
		server <- dynamicWorkloadServerResult{workload: workload, err: serveErr}
	}()

	workload, err := exerciseConfiguredDynamic(client, "http://reference.ard/", plan, publisherTerminalApplicationReset, "")
	if err != nil {
		t.Fatal(err)
	}
	result := <-server
	if result.err == nil || !strings.Contains(result.err.Error(), "simulated Publisher Application crash after configured warmup") {
		t.Fatalf("dynamic Application terminal = %v", result.err)
	}
	if workload.CompletedCycles != 2 || !workload.TerminalNoFallback || result.workload.CompletedCycles != 2 {
		t.Fatalf("dynamic Application-loss progress = user %+v / server %+v", workload, result.workload)
	}
	if contents, err := os.ReadFile(proof); err != nil || string(contents) != "dynamic-application-crash\n" {
		t.Fatalf("dynamic Application-loss proof = %q / %v", contents, err)
	}
}

func TestConfiguredDynamicApplicationLossWaitsForExplicitInjection(t *testing.T) {
	root := t.TempDir()
	proof, ready, release := filepath.Join(root, "proof"), filepath.Join(root, "application-fault-ready"), filepath.Join(root, "application-fault-release")
	plan := dynamicWorkloadPlan{cycles: 1, interval: 20 * time.Millisecond, cycleDeadline: time.Second}
	client, accept := dynamicWorkloadTestClient(t)
	server := make(chan dynamicWorkloadServerResult, 1)
	go func() {
		connection, err := accept()
		if err != nil {
			server <- dynamicWorkloadServerResult{err: err}
			return
		}
		controls := dynamicWorkloadControls{applicationFaultReady: ready, applicationFaultRelease: release}
		workload, serveErr := serveConfiguredDynamic(connection, proof, controls, plan, publisherTerminalApplicationReset, "")
		server <- dynamicWorkloadServerResult{workload: workload, err: serveErr}
	}()
	user := make(chan error, 1)
	go func() {
		_, err := exerciseConfiguredDynamic(client, "http://reference.ard/", plan, publisherTerminalApplicationReset, "")
		user <- err
	}()
	waitForDynamicTestFile(t, ready)
	select {
	case err := <-user:
		t.Fatalf("Application fault completed before explicit injection: %v", err)
	default:
	}
	releaseTemporary := release + ".tmp"
	if err := os.WriteFile(releaseTemporary, []byte("inject\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(releaseTemporary, release); err != nil {
		t.Fatal(err)
	}
	if err := <-user; err != nil {
		t.Fatal(err)
	}
	result := <-server
	if result.err == nil || !strings.Contains(result.err.Error(), "simulated Publisher Application crash") {
		t.Fatalf("gated Application terminal = %v", result.err)
	}
}

func TestValidatePublisherApplicationFaultControlFailsClosed(t *testing.T) {
	root := t.TempDir()
	valid := config{TransparentApplication: true, PublisherTerminal: publisherTerminalApplicationReset,
		PublisherApplicationFaultReadyPath: filepath.Join(root, "ready"), PublisherApplicationFaultReleasePath: filepath.Join(root, "release"),
		DynamicWorkload: dynamicWorkloadConfig{Cycles: 1}}
	if err := validatePublisherApplicationFaultControl(valid); err != nil {
		t.Fatalf("valid Application fault control was rejected: %v", err)
	}
	for name, mutate := range map[string]func(*config){
		"missing-release": func(input *config) { input.PublisherApplicationFaultReleasePath = "" },
		"same-path": func(input *config) {
			input.PublisherApplicationFaultReleasePath = input.PublisherApplicationFaultReadyPath
		},
		"relative":       func(input *config) { input.PublisherApplicationFaultReadyPath = "relative" },
		"wrong-terminal": func(input *config) { input.PublisherTerminal = "" },
		"no-workload":    func(input *config) { input.DynamicWorkload = dynamicWorkloadConfig{} },
	} {
		t.Run(name, func(t *testing.T) {
			input := valid
			mutate(&input)
			if err := validatePublisherApplicationFaultControl(input); err == nil {
				t.Fatal("invalid Application fault control was accepted")
			}
		})
	}
	if err := validatePublisherApplicationFaultControl(config{}); err != nil {
		t.Fatalf("omitted Application fault control changed existing behavior: %v", err)
	}
}

func TestConfiguredDynamicEndpointLossFollowsWarmupWithoutReplay(t *testing.T) {
	root := t.TempDir()
	proof, crashReady := filepath.Join(root, "proof"), filepath.Join(root, "crash-ready")
	plan := dynamicWorkloadPlan{cycles: 2, interval: 50 * time.Millisecond, cycleDeadline: time.Second}
	client, accept := dynamicWorkloadTestClient(t)
	server := make(chan dynamicWorkloadServerResult, 1)
	closerContext, cancelCloser := context.WithCancel(t.Context())
	defer cancelCloser()
	go func() {
		accepted, err := accept()
		if err != nil {
			server <- dynamicWorkloadServerResult{err: err}
			return
		}
		closerDone := make(chan struct{})
		go closeDynamicWorkloadEndpointAtReady(closerContext, accepted, crashReady, closerDone)
		workload, serveErr := serveConfiguredDynamic(accepted, proof, dynamicWorkloadControls{endpointCrashReady: crashReady}, plan, publisherTerminalEndpointStop, "")
		cancelCloser()
		<-closerDone
		server <- dynamicWorkloadServerResult{workload: workload, err: serveErr}
	}()

	workload, err := exerciseConfiguredDynamic(client, "http://reference.ard/", plan, publisherTerminalEndpointStop, "")
	if err != nil {
		t.Fatal(err)
	}
	result := <-server
	if result.err == nil || !strings.Contains(result.err.Error(), "simulated Publisher Endpoint crash closed the local Application handoff") {
		t.Fatalf("dynamic Endpoint terminal = %v", result.err)
	}
	if workload.CompletedCycles != 2 || !workload.TerminalNoFallback || result.workload.CompletedCycles != 2 {
		t.Fatalf("dynamic Endpoint-loss progress = user %+v / server %+v", workload, result.workload)
	}
}

func TestDynamicWorkloadEndpointCloserCanBeCanceledAndJoined(t *testing.T) {
	connection, peer := net.Pipe()
	defer peer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go closeDynamicWorkloadEndpointAtReady(ctx, connection, filepath.Join(t.TempDir(), "not-ready"), done)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled dynamic Endpoint closer did not terminate")
	}
}

func closeDynamicWorkloadEndpointAtReady(ctx context.Context, connection net.Conn, readyPath string, done chan<- struct{}) {
	defer close(done)
	defer connection.Close()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		if _, err := os.Stat(readyPath); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-timer.C:
			return
		}
	}
}

func waitForDynamicTestFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(path); err == nil && string(raw) == "ready\n" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("dynamic test file %s did not become ready", path)
}

func TestDynamicWorkloadConfigKeepsQualificationLoadBounded(t *testing.T) {
	valid := config{TransparentApplication: true, Deadline: time.Now().UTC().Add(31 * time.Minute).Truncate(time.Second).Format(time.RFC3339),
		DynamicWorkload: dynamicWorkloadConfig{Cycles: 1800, IntervalMilliseconds: 1_000, CycleDeadlineMilliseconds: 5_000,
			NoFallbackEvery: 60, BytesEachDirection: 4 << 20}}
	if err := valid.DynamicWorkload.validate(valid); err != nil {
		t.Fatalf("A11 dynamic workload contract was rejected: %v", err)
	}
	explicit := valid
	explicit.DynamicWorkload.MaximumStartLagMilliseconds = 5_000
	if err := explicit.DynamicWorkload.validate(explicit); err != nil {
		t.Fatalf("bounded explicit start-lag limit was rejected: %v", err)
	}
	invalid := valid
	invalid.DynamicWorkload.Cycles++
	if err := invalid.DynamicWorkload.validate(invalid); err == nil {
		t.Fatal("dynamic workload admitted more than 1,800 cycles")
	}
	invalid = valid
	invalid.DynamicWorkload.NoFallbackEvery = 0
	if err := invalid.DynamicWorkload.validate(invalid); err == nil {
		t.Fatal("dynamic workload admitted no periodic no-fallback probe")
	}
	invalid = valid
	invalid.DynamicWorkload.CycleDeadlineMilliseconds++
	if err := invalid.DynamicWorkload.validate(invalid); err == nil {
		t.Fatal("dynamic workload admitted a cycle deadline above five seconds")
	}
	invalid = valid
	invalid.DynamicWorkload.MaximumStartLagMilliseconds = 999
	if err := invalid.DynamicWorkload.validate(invalid); err == nil {
		t.Fatal("dynamic workload admitted a start-lag limit below its cadence")
	}
	invalid = valid
	invalid.DynamicWorkload.MaximumStartLagMilliseconds = 5_001
	if err := invalid.DynamicWorkload.validate(invalid); err == nil {
		t.Fatal("dynamic workload admitted a start-lag limit above its request deadline")
	}
}

func TestConfiguredDynamicWorkloadRejectsLateFirstCycle(t *testing.T) {
	plan := dynamicWorkloadPlan{cycles: 1, interval: time.Nanosecond, cycleDeadline: time.Second}
	_, err := exerciseConfiguredDynamic(&http.Client{}, "http://reference.ard/", plan, "", "")
	if err == nil || !strings.Contains(err.Error(), "missed a pacing slot") {
		t.Fatalf("late first cycle = %v, want pacing failure before any dial", err)
	}
}

func TestDynamicWorkloadSeparatesCompressedCadenceFromStartLagLimit(t *testing.T) {
	configured := dynamicWorkloadConfig{IntervalMilliseconds: 50, MaximumStartLagMilliseconds: 250}
	plan := configured.plan()
	if plan.interval != 50*time.Millisecond || plan.maximumStartLag != 250*time.Millisecond {
		t.Fatalf("compressed plan cadence/lag = %s/%s, want 50ms/250ms", plan.interval, plan.maximumStartLag)
	}
	if missedDynamicPacingSlot(60*time.Millisecond, plan.maximumStartLag) {
		t.Fatal("declared scheduler tolerance rejected a healthy compressed workload")
	}
	if !missedDynamicPacingSlot(250*time.Millisecond, plan.maximumStartLag) {
		t.Fatal("declared scheduler tolerance admitted a complete pacing overrun")
	}
	legacy := (dynamicWorkloadConfig{IntervalMilliseconds: 1_000}).plan()
	if legacy.maximumStartLag != time.Second {
		t.Fatalf("default qualification lag limit = %s, want one interval", legacy.maximumStartLag)
	}
}

type dynamicWorkloadServerResult struct {
	workload dynamicWorkloadResult
	err      error
}

func dynamicWorkloadTestClient(t *testing.T) (*http.Client, func() (net.Conn, error)) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	dialer := &singleDynamicWorkloadDialer{address: listener.Addr().String()}
	transport := &http.Transport{DialContext: dialer.dial}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport}, listener.Accept
}

type singleDynamicWorkloadDialer struct {
	mu      sync.Mutex
	address string
	dialed  bool
}

func (dialer *singleDynamicWorkloadDialer) dial(ctx context.Context, network, _ string) (net.Conn, error) {
	dialer.mu.Lock()
	if dialer.dialed {
		dialer.mu.Unlock()
		return nil, errors.New("configured dynamic workload refused a reconnect")
	}
	dialer.dialed = true
	dialer.mu.Unlock()
	var connection net.Dialer
	return connection.DialContext(ctx, network, dialer.address)
}
