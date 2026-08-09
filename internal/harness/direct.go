package harness

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

const directControlSchema = "carrier-lab-direct-control/v1"

type directControlSummary struct {
	SchemaVersion               string                       `json:"schema_version"`
	RunID                       string                       `json:"run_id"`
	Status                      string                       `json:"status"`
	BinarySHA256                string                       `json:"binary_sha256"`
	Cases                       map[string]directCaseSummary `json:"cases"`
	Checks                      map[string]bool              `json:"checks"`
	DirectRelationshipDisclosed bool                         `json:"direct_relationship_disclosed"`
	RouteFallback               bool                         `json:"route_fallback"`
	ElapsedMilliseconds         int64                        `json:"elapsed_milliseconds"`
	Failure                     string                       `json:"failure,omitempty"`
}

type directCaseSummary struct {
	Expected                   string `json:"expected"`
	Observed                   string `json:"observed"`
	UserExit                   int    `json:"user_exit"`
	ServiceExit                int    `json:"service_exit"`
	ProxyExit                  int    `json:"proxy_exit,omitempty"`
	UserElapsedMilliseconds    int64  `json:"user_elapsed_milliseconds"`
	ServiceElapsedMilliseconds int64  `json:"service_elapsed_milliseconds"`
	UserHeapAllocBytes         uint64 `json:"user_heap_alloc_bytes"`
	ServiceHeapAllocBytes      uint64 `json:"service_heap_alloc_bytes"`
	UserGoroutines             int    `json:"user_goroutines"`
	ServiceGoroutines          int    `json:"service_goroutines"`
	ProxyHeapAllocBytes        uint64 `json:"proxy_heap_alloc_bytes,omitempty"`
	PayloadBytes               int    `json:"payload_bytes,omitempty"`
	Passed                     bool   `json:"passed"`
}

type directEvidenceResult struct {
	Status                   string `json:"status"`
	TerminalResult           string `json:"terminal_result"`
	TLSVersion               string `json:"tls_version"`
	Curve                    string `json:"curve"`
	SessionResumed           bool   `json:"session_resumed"`
	ApplicationBytesVerified bool   `json:"application_bytes_verified"`
	ProtectedRecordModified  bool   `json:"protected_record_modified"`
	PayloadBytes             int    `json:"payload_bytes"`
	ElapsedMilliseconds      int64  `json:"elapsed_milliseconds"`
	HeapAllocBytes           uint64 `json:"heap_alloc_bytes"`
	Goroutines               int    `json:"goroutines"`
}

// RunDirectControl owns the complete fixed Direct TLS measurement control:
// fixtures, child processes, three cases, bounded evidence, and cleanup.
func RunDirectControl(ctx context.Context, identity preflight.RunLayout, binaryPath string) (evidenceDir string, runErr error) {
	layout, err := ownedLayout(identity, false, false)
	if err != nil {
		return "", err
	}
	if err := requireDirectBinary(binaryPath); err != nil {
		return "", err
	}
	if err := prepareSmokeWorkspace(layout); err != nil {
		return "", err
	}
	evidenceDir = layout.evidenceDir
	started := time.Now()
	summary := directControlSummary{
		SchemaVersion: directControlSchema, RunID: layout.runID, Status: "failed", Cases: make(map[string]directCaseSummary, 3),
		Checks: map[string]bool{
			"exact_target_instance_authenticated": false, "positive_canary_verified": false,
			"wrong_instance_failed_closed": false, "modified_record_failed_closed": false,
			"case_process_outcomes": true, "resource_evidence_retained": true,
			"processes_reaped": false, "cleanup_complete": false,
		},
		DirectRelationshipDisclosed: true, RouteFallback: false,
	}
	defer func() {
		runErr = finishDirectControl(layout, &summary, started, runErr)
	}()
	digest, err := hashDirectBinary(binaryPath)
	if err != nil {
		return evidenceDir, err
	}
	summary.BinarySHA256 = digest
	fixture, err := prepareDirectFixture(layout.runDir)
	if err != nil {
		return evidenceDir, err
	}
	for _, caseName := range []string{"positive", "wrong-instance", "modified-record"} {
		caseSummary, roleEvidence, err := runDirectCase(ctx, layout, binaryPath, fixture, caseName)
		summary.Cases[caseName] = caseSummary
		if err != nil {
			return evidenceDir, fmt.Errorf("direct TLS case %s: %w", caseName, err)
		}
		summary.Checks["case_process_outcomes"] = summary.Checks["case_process_outcomes"] && caseSummary.Passed
		summary.Checks["resource_evidence_retained"] = summary.Checks["resource_evidence_retained"] &&
			caseSummary.UserHeapAllocBytes > 0 && caseSummary.ServiceHeapAllocBytes > 0 &&
			caseSummary.UserGoroutines > 0 && caseSummary.ServiceGoroutines > 0
		switch caseName {
		case "positive":
			summary.Checks["exact_target_instance_authenticated"] = roleEvidence.Status == "passed" && roleEvidence.TLSVersion == "TLS1.3" && roleEvidence.Curve == "X25519" && !roleEvidence.SessionResumed
			summary.Checks["positive_canary_verified"] = roleEvidence.ApplicationBytesVerified
		case "wrong-instance":
			summary.Checks["wrong_instance_failed_closed"] = roleEvidence.Status == "failed" && roleEvidence.TerminalResult == "explicit_failure" && !roleEvidence.ApplicationBytesVerified
		case "modified-record":
			summary.Checks["modified_record_failed_closed"] = roleEvidence.Status == "failed" && roleEvidence.TerminalResult == "explicit_failure" && !roleEvidence.ApplicationBytesVerified && caseSummary.ProxyExit == 0
		}
	}
	summary.Checks["processes_reaped"] = true
	if !allChecksPassed(summary.Checks, "cleanup_complete") {
		return evidenceDir, errors.New("direct TLS control checks failed")
	}
	summary.Status = "passed"
	return evidenceDir, nil
}

func runDirectCase(ctx context.Context, layout runLayout, binaryPath string, fixture directFixture, caseName string) (directCaseSummary, directEvidenceResult, error) {
	caseRoot := filepath.Join(layout.runDir, "direct-"+caseName)
	serviceEvidence := filepath.Join(caseRoot, "service-evidence")
	userEvidence := filepath.Join(caseRoot, "user-evidence")
	proxyEvidence := filepath.Join(caseRoot, "proxy-evidence")
	for _, directory := range []string{caseRoot, serviceEvidence, userEvidence, proxyEvidence} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return directCaseSummary{}, directEvidenceResult{}, err
		}
	}
	serviceAddress, err := reserveDirectAddress()
	if err != nil {
		return directCaseSummary{}, directEvidenceResult{}, err
	}
	userAddress := serviceAddress
	proxyAddress := ""
	if caseName == "modified-record" {
		proxyAddress, err = reserveDirectAddress()
		if err != nil {
			return directCaseSummary{}, directEvidenceResult{}, err
		}
		userAddress = proxyAddress
	}
	certificatePath, privateKeyPath := fixture.activeCertificate, fixture.activePrivateKey
	if caseName == "wrong-instance" {
		certificatePath, privateKeyPath = fixture.wrongCertificate, fixture.wrongPrivateKey
	}
	serviceConfig := filepath.Join(caseRoot, "service.json")
	userConfig := filepath.Join(caseRoot, "user.json")
	if err := writeBoundedJSON(serviceConfig, directRoleConfigInput{
		SchemaVersion: "carrier-lab-direct-role/v1", RunID: layout.runID, Case: caseName, Role: "service", Address: serviceAddress,
		CertificatePath: certificatePath, PrivateKeyPath: privateKeyPath,
	}); err != nil {
		return directCaseSummary{}, directEvidenceResult{}, err
	}
	if err := writeBoundedJSON(userConfig, directRoleConfigInput{
		SchemaVersion: "carrier-lab-direct-role/v1", RunID: layout.runID, Case: caseName, Role: "user", Address: userAddress,
		TargetRootPath: fixture.targetRoot, ExpectedLeafSHA256: fixture.activeLeafSHA256,
		CanaryHex: fixture.canaryHex, PayloadSeed: fixture.payloadSeed, PayloadSize: 64 * 1024,
	}); err != nil {
		return directCaseSummary{}, directEvidenceResult{}, err
	}
	service := startDirectChild(ctx, binaryPath, "direct-role", "--config", serviceConfig, "--evidence-dir", serviceEvidence)
	if service.startErr != nil {
		return directCaseSummary{}, directEvidenceResult{}, service.startErr
	}
	defer service.stop()
	if err := waitDirectReady(ctx, filepath.Join(serviceEvidence, "ready.json")); err != nil {
		return directCaseSummary{}, directEvidenceResult{}, err
	}
	var proxy *directChild
	if caseName == "modified-record" {
		proxyConfig := filepath.Join(caseRoot, "proxy.json")
		if err := writeBoundedJSON(proxyConfig, directTamperConfigInput{
			SchemaVersion: "carrier-lab-direct-tamper/v1", RunID: layout.runID, ListenAddress: proxyAddress, ServiceAddress: serviceAddress,
		}); err != nil {
			return directCaseSummary{}, directEvidenceResult{}, err
		}
		child := startDirectChild(ctx, binaryPath, "direct-tamper", "--config", proxyConfig, "--evidence-dir", proxyEvidence)
		proxy = &child
		if proxy.startErr != nil {
			return directCaseSummary{}, directEvidenceResult{}, proxy.startErr
		}
		defer proxy.stop()
		if err := waitDirectReady(ctx, filepath.Join(proxyEvidence, "ready.json")); err != nil {
			return directCaseSummary{}, directEvidenceResult{}, err
		}
	}
	user := startDirectChild(ctx, binaryPath, "direct-role", "--config", userConfig, "--evidence-dir", userEvidence)
	userExit := user.wait()
	serviceExit := service.wait()
	proxyExit := 0
	if proxy != nil {
		proxyExit = proxy.wait()
	}
	var userResult directEvidenceResult
	if err := readDirectEvidence(filepath.Join(userEvidence, "result.json"), &userResult); err != nil {
		return directCaseSummary{}, directEvidenceResult{}, err
	}
	var serviceResult directEvidenceResult
	if err := readDirectEvidence(filepath.Join(serviceEvidence, "result.json"), &serviceResult); err != nil {
		return directCaseSummary{}, directEvidenceResult{}, err
	}
	var proxyResult directEvidenceResult
	if proxy != nil {
		if err := readDirectEvidence(filepath.Join(proxyEvidence, "result.json"), &proxyResult); err != nil {
			return directCaseSummary{}, directEvidenceResult{}, err
		}
	}
	passed := userExit == 0 && serviceExit == 0
	expected := "success"
	observed := "success"
	if caseName != "positive" {
		expected = "explicit_failure"
		observed = userResult.TerminalResult
		passed = userExit != 0 && userResult.Status == "failed" && !userResult.ApplicationBytesVerified
		if caseName == "modified-record" {
			passed = passed && proxyExit == 0
		}
	}
	return directCaseSummary{
		Expected: expected, Observed: observed, UserExit: userExit, ServiceExit: serviceExit, ProxyExit: proxyExit,
		UserElapsedMilliseconds: userResult.ElapsedMilliseconds, ServiceElapsedMilliseconds: serviceResult.ElapsedMilliseconds,
		UserHeapAllocBytes: userResult.HeapAllocBytes, ServiceHeapAllocBytes: serviceResult.HeapAllocBytes,
		UserGoroutines: userResult.Goroutines, ServiceGoroutines: serviceResult.Goroutines,
		ProxyHeapAllocBytes: proxyResult.HeapAllocBytes, PayloadBytes: userResult.PayloadBytes, Passed: passed,
	}, userResult, nil
}

func finishDirectControl(layout runLayout, summary *directControlSummary, started time.Time, runErr error) error {
	summary.ElapsedMilliseconds = time.Since(started).Milliseconds()
	if runErr != nil {
		summary.Status = "failed"
		summary.Failure = runErr.Error()
	}
	cleanupErr := removeSmokeRunDirectory(layout)
	if cleanupErr == nil {
		summary.Checks["cleanup_complete"] = true
	} else {
		summary.Status = "failed"
		summary.Failure = errors.Join(runErr, cleanupErr).Error()
	}
	evidenceErr := writeBoundedJSON(filepath.Join(layout.evidenceDir, "direct-control.json"), summary)
	return errors.Join(runErr, cleanupErr, evidenceErr)
}

func requireDirectBinary(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("binary path must be absolute and clean")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("binary path is not a regular file")
	}
	return nil
}

func hashDirectBinary(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func reserveDirectAddress() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	address := listener.Addr().String()
	return address, listener.Close()
}

func waitDirectReady(ctx context.Context, path string) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Size() <= smokeEvidenceCap {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	return errors.New("direct TLS child did not become ready")
}

func readDirectEvidence(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) > smokeEvidenceCap {
		return errors.New("direct TLS role evidence exceeds its cap")
	}
	return json.Unmarshal(data, target)
}

type directChild struct {
	command  *exec.Cmd
	output   bytes.Buffer
	startErr error
	waited   bool
	exitCode int
}

func startDirectChild(ctx context.Context, binaryPath string, arguments ...string) directChild {
	child := directChild{command: exec.CommandContext(ctx, binaryPath, arguments...)}
	child.command.Stdout = &child.output
	child.command.Stderr = &child.output
	child.startErr = child.command.Start()
	return child
}

func (child *directChild) wait() int {
	if child == nil || child.startErr != nil {
		return -1
	}
	if child.waited {
		return child.exitCode
	}
	err := child.command.Wait()
	child.waited = true
	child.exitCode = 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			child.exitCode = exitError.ExitCode()
		} else {
			child.exitCode = -1
		}
	}
	return child.exitCode
}

func (child *directChild) stop() {
	if child == nil || child.startErr != nil || child.waited {
		return
	}
	_ = child.command.Process.Kill()
	child.wait()
}
