//go:build ignore

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const artifactsDir = "/workspace/artifacts"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "test-driver:", err)
		os.Exit(2)
	}
}

func run(ctx context.Context, arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: test-driver <prebake|verify|self-test> EVIDENCE_DIR")
	}
	switch arguments[0] {
	case "prebake":
		return runPrebake(ctx, arguments[1:])
	case "verify":
		return runVerify(ctx, arguments[1:])
	case "self-test":
		return runSelfTest(ctx)
	default:
		return fmt.Errorf("unknown subcommand %q", arguments[0])
	}
}

func runPrebake(ctx context.Context, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("usage: test-driver prebake EVIDENCE_DIR")
	}
	evidenceDir, err := filepath.Abs(arguments[0])
	if err != nil {
		return fmt.Errorf("evidence dir abs: %w", err)
	}
	if err := os.MkdirAll(evidenceDir, 0o700); err != nil {
		return fmt.Errorf("mkdir evidence: %w", err)
	}
	fixturesDir := filepath.Join(evidenceDir, "fixtures")
	if err := os.MkdirAll(fixturesDir, 0o700); err != nil {
		return fmt.Errorf("mkdir fixtures: %w", err)
	}
	sourceAState := filepath.Join(evidenceDir, "source-a-state")
	sourceBState := filepath.Join(evidenceDir, "source-b-state")
	now := time.Now().UTC()
	fixtures, err := Prebake(fixturesDir, now)
	if err != nil {
		return fmt.Errorf("prebake: %w", err)
	}
	clientPin, sourceAPin, sourceBPin, err := WriteCerts(
		filepath.Join(fixturesDir, "source-ca.pem"),
		filepath.Join(fixturesDir, "source-a.pem"), filepath.Join(fixturesDir, "source-a-key.pem"),
		filepath.Join(fixturesDir, "source-b.pem"), filepath.Join(fixturesDir, "source-b-key.pem"),
		filepath.Join(fixturesDir, "client-ca.pem"),
		filepath.Join(fixturesDir, "client.pem"), filepath.Join(fixturesDir, "client-key.pem"),
		now)
	if err != nil {
		return fmt.Errorf("prebake certs: %w", err)
	}
	if err := WritePlans(fixturesDir, sourceAState, sourceBState,
		ResolveSourceAddress("PILOT_SOURCE_A_ADDRESS", DefaultSourceAddressA),
		ResolveSourceAddress("PILOT_SOURCE_B_ADDRESS", DefaultSourceAddressB),
		fixtures, clientPin, sourceAPin, sourceBPin, now); err != nil {
		return fmt.Errorf("prebake plans: %w", err)
	}
	if err := acceptOffline(evidenceDir, fixturesDir, sourceAState, fixtures, now); err != nil {
		return fmt.Errorf("accept-offline source-a-state: %w", err)
	}
	if err := acceptOffline(evidenceDir, fixturesDir, sourceBState, fixtures, now); err != nil {
		return fmt.Errorf("accept-offline source-b-state: %w", err)
	}
	fmt.Printf("test-driver: prebake complete generation=%s network_id=%s client_pin=%x source-a-state=%s source-b-state=%s\n",
		fixtures.Generation, fixtures.NetworkIDHex, clientPin, sourceAState, sourceBState)
	_ = ctx
	return nil
}

func acceptOffline(evidenceDir, fixturesDir, stateRoot string, fixtures Fixtures, now time.Time) error {
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		return fmt.Errorf("mkdir state root: %w", err)
	}
	authorityHex := fmt.Sprintf("%x", fixtures.AuthorityPublic)
	generation := fixtures.Generation
	materializationPath := filepath.Join(fixturesDir, "materialization.bin")
	if err := os.WriteFile(materializationPath, fixtures.Materialization, 0o600); err != nil {
		return fmt.Errorf("write materialization: %w", err)
	}
	args := []string{
		"accept-offline",
		"--state-root", stateRoot,
		"--network-id", fixtures.NetworkIDHex,
		"--authorities", authorityHex,
		"--threshold", "1",
		"--at", now.UTC().Format(time.RFC3339),
		"--epoch", filepath.Join(fixturesDir, "generations", generation, "epoch.bin"),
		"--inputs", filepath.Join(fixturesDir, "generations", generation, "inputs"),
		"--materialization", materializationPath,
	}
	ardents := filepath.Join(artifactsDir, "ardents-linux-amd64")
	logPath := filepath.Join(evidenceDir, "logs", "accept-offline-"+filepath.Base(stateRoot)+".log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return fmt.Errorf("mkdir logs: %w", err)
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()
	command := exec.Command(ardents, args...)
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s: %w (see %s)", ardents, err, logPath)
	}
	return nil
}

func runVerify(ctx context.Context, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("usage: test-driver verify EVIDENCE_DIR")
	}
	evidenceDir, err := filepath.Abs(arguments[0])
	if err != nil {
		return fmt.Errorf("evidence dir abs: %w", err)
	}
	generationPath := filepath.Join(evidenceDir, "fixtures", "current")
	raw, err := os.ReadFile(generationPath)
	if err != nil {
		return fmt.Errorf("read expected generation: %w", err)
	}
	expected := string(raw)
	if len(expected) > 0 && expected[len(expected)-1] == '\n' {
		expected = expected[:len(expected)-1]
	}
	verdict, err := VerifyConvergence(evidenceDir, expected)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	fmt.Printf("test-driver: verify accept=%v distinct=%d reason=%q\n",
		verdict.Accept, verdict.DistinctResults, verdict.Reason)
	_ = ctx
	if !verdict.Accept {
		os.Exit(3)
	}
	return nil
}

func runSelfTest(ctx context.Context) error {
	tmp, err := os.MkdirTemp("", "pilot-self-test-")
	if err != nil {
		return fmt.Errorf("mktemp: %w", err)
	}
	defer os.RemoveAll(tmp)
	fixturesDir := filepath.Join(tmp, "fixtures")
	if err := os.MkdirAll(fixturesDir, 0o700); err != nil {
		return fmt.Errorf("mkdir fixtures: %w", err)
	}
	now := time.Now().UTC()
	fixtures, err := Prebake(fixturesDir, now)
	if err != nil {
		return fmt.Errorf("self-test prebake: %w", err)
	}
	clientPin, sourceAPin, sourceBPin, err := WriteCerts(
		filepath.Join(fixturesDir, "source-ca.pem"),
		filepath.Join(fixturesDir, "source-a.pem"), filepath.Join(fixturesDir, "source-a-key.pem"),
		filepath.Join(fixturesDir, "source-b.pem"), filepath.Join(fixturesDir, "source-b-key.pem"),
		filepath.Join(fixturesDir, "client-ca.pem"),
		filepath.Join(fixturesDir, "client.pem"), filepath.Join(fixturesDir, "client-key.pem"),
		now)
	if err != nil {
		return fmt.Errorf("self-test certs: %w", err)
	}
	if err := WritePlans(fixturesDir, filepath.Join(tmp, "source-a-state"),
		filepath.Join(tmp, "source-b-state"), DefaultSourceAddressA, DefaultSourceAddressB,
		fixtures, clientPin, sourceAPin, sourceBPin, now); err != nil {
		return fmt.Errorf("self-test plans: %w", err)
	}
	nodesDir := filepath.Join(tmp, "nodes")
	if err := os.MkdirAll(nodesDir, 0o700); err != nil {
		return fmt.Errorf("self-test mkdir nodes: %w", err)
	}
	for index := 0; index < 6; index++ {
		raw := fmt.Sprintf(`{"schema":"ardents-source-event-v1","kind":"source-wave-accepted","generation":"%s","epoch":%d,"source_attempts":2,"source_outcomes":["valid","valid","not-attempted","not-attempted"],"latest_completeness":"latest completeness unproven"}`,
			fixtures.Generation, fixtures.EpochNumber)
		if err := os.WriteFile(filepath.Join(nodesDir, fmt.Sprintf("node-%d.json", index+1)),
			[]byte(raw+"\n"), 0o600); err != nil {
			return fmt.Errorf("self-test write node log: %w", err)
		}
	}
	verdict, err := VerifyConvergence(tmp, fixtures.Generation)
	if err != nil {
		return fmt.Errorf("self-test verify: %w", err)
	}
	if !verdict.Accept {
		return fmt.Errorf("self-test: synthetic convergence verdict should accept, got reason %q", verdict.Reason)
	}
	if verdict.DistinctResults != 1 {
		return fmt.Errorf("self-test: synthetic verdict should be 1 distinct set, got %d", verdict.DistinctResults)
	}
	fmt.Println("test-driver: self-test passed")
	_ = ctx
	return nil
}
