package tooling

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	toolingRoleSchema   = "carrier-lab-tooling-role/v1"
	toolingResultSchema = "carrier-lab-tooling-result/v1"
	toolingMarkerPrefix = "carrier-lab-tooling-tracer/"
	toolingTCPPort      = "37002"
	toolingLockPath     = "/usr/share/ardents/carrier-lab-tools.lock"
)

var (
	tcVersionPattern      = regexp.MustCompile(`iproute2-([0-9]+(?:\.[0-9]+)+)`)
	tcpdumpVersionPattern = regexp.MustCompile(`(?m)^tcpdump version ([^\s]+)`)
	libpcapVersionPattern = regexp.MustCompile(`(?m)^libpcap version ([^\s]+)`)
)

type observedTool struct {
	Version string `json:"version"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
}

type toolingRoleResult struct {
	SchemaVersion         string                  `json:"schema_version"`
	RunID                 string                  `json:"run_id"`
	Role                  string                  `json:"role"`
	Status                string                  `json:"status"`
	EffectiveCapabilities string                  `json:"effective_capabilities"`
	ToolLockSHA256        string                  `json:"tool_lock_sha256,omitempty"`
	Tools                 map[string]observedTool `json:"tools,omitempty"`
	QdiscState            string                  `json:"qdisc_state,omitempty"`
	CaptureSHA256         string                  `json:"capture_sha256,omitempty"`
	CaptureBytes          int64                   `json:"capture_bytes,omitempty"`
	CaptureTracer         bool                    `json:"capture_contains_tracer,omitempty"`
	RawCaptureRemoved     bool                    `json:"raw_capture_removed,omitempty"`
	ObservedPeer          string                  `json:"observed_peer,omitempty"`
	Failure               string                  `json:"failure,omitempty"`
}

// RunRole dispatches one fixed laboratory role without exposing its
// shaping, capture, or tracer implementation seams to product code.
func RunRole(kind, runID, role, fault string) error {
	switch kind {
	case "tracer":
		if fault != "" {
			return errors.New("tracer role does not accept fault injection")
		}
		return runToolTracerRole(runID, role, "/control", "/evidence")
	case "shape":
		return runToolShaperRole(runID, role, "eth0", "/control", "/evidence", fault)
	case "capture":
		if role != "alpha" {
			return errors.New("capture is fixed to the alpha laboratory link")
		}
		return runToolCaptureRole(runID, "eth0", "/evidence", "/capture/alpha-link.pcap", fault)
	default:
		return errors.New("unknown tooling role kind")
	}
}

func runToolShaperRole(runID, role, networkInterface, controlDirectory, evidenceDirectory, fault string) error {
	if !validToolRole(runID, role) || fault != "" && fault != "shaping" {
		return errors.New("invalid tooling shaper role input")
	}
	result := newToolingRoleResult(runID, "shape-"+role)
	finish := func(runErr error) error { return finishToolingRole(evidenceDirectory, &result, runErr) }
	capabilities, err := effectiveCapabilities()
	result.EffectiveCapabilities = capabilities
	if err != nil || !hasOnlyEffectiveCapability(capabilities, capabilityNetAdmin) {
		return finish(errors.Join(err, errors.New("shaping role requires exactly NET_ADMIN")))
	}
	identity, tools, err := observeTools(toolingLockPath, "tc")
	if err != nil {
		return finish(err)
	}
	result.ToolLockSHA256, result.Tools = identity.LockSHA256, tools
	if fault == "shaping" {
		return finish(errors.New("injected shaping failure"))
	}
	run := func(name string, arguments ...string) ([]byte, error) {
		return exec.Command(name, arguments...).CombinedOutput()
	}
	state, err := applyAndObserveShaping(run, networkInterface)
	if err != nil {
		return finish(err)
	}
	result.QdiscState = strings.TrimSpace(state)
	if err := writeToolingReady(evidenceDirectory, runID, result.Role); err != nil {
		_ = deleteShaping(run, networkInterface)
		return finish(err)
	}
	if err := waitForControlFile(filepath.Join(controlDirectory, "stop"), 20*time.Second); err != nil {
		_ = deleteShaping(run, networkInterface)
		return finish(err)
	}
	if err := deleteShaping(run, networkInterface); err != nil {
		return finish(err)
	}
	return finish(nil)
}

// RunToolCaptureRole records only the fixed TCP tracer on the designated link.
// It retains the hash and bounded decoded assertion, then deletes the raw pcap.
func runToolCaptureRole(runID, networkInterface, evidenceDirectory, capturePath, fault string) (runErr error) {
	result := newToolingRoleResult(runID, "capture-alpha")
	finish := func(err error) error { return finishToolingRole(evidenceDirectory, &result, err) }
	if !runIDPattern.MatchString(runID) || fault != "" && fault != "capture-start" || capturePath != "/capture/alpha-link.pcap" {
		return finish(errors.New("invalid tooling capture role input"))
	}
	defer func() {
		cleanupErr := removeRawCapture(capturePath, filepath.Dir(capturePath))
		result.RawCaptureRemoved = cleanupErr == nil
		runErr = errors.Join(runErr, cleanupErr)
	}()
	capabilities, err := effectiveCapabilities()
	result.EffectiveCapabilities = capabilities
	if err != nil || !hasOnlyEffectiveCapability(capabilities, capabilityNetRaw) {
		return finish(errors.Join(err, errors.New("capture role requires exactly NET_RAW")))
	}
	identity, tools, err := observeTools(toolingLockPath, "tcpdump", "libpcap")
	if err != nil {
		return finish(err)
	}
	result.ToolLockSHA256, result.Tools = identity.LockSHA256, tools
	if fault == "capture-start" {
		return finish(errors.New("injected capture startup failure"))
	}
	if err := os.Remove(capturePath); err != nil && !os.IsNotExist(err) {
		return finish(err)
	}
	command := exec.Command("/usr/bin/tcpdump", "-Z", "root", "-i", networkInterface, "-n", "-U", "-s", "256", "-B", "1024", "-c", "12", "-w", capturePath, "tcp", "port", toolingTCPPort)
	stderrPipe, startErr := command.StderrPipe()
	waited := make(chan error, 1)
	ready := make(chan struct{}, 1)
	stderrFinished := make(chan string, 1)
	if startErr == nil {
		startErr = command.Start()
	}
	if startErr == nil {
		go scanCaptureStderr(stderrPipe, ready, stderrFinished)
		go func() { waited <- command.Wait() }()
	}
	if err := validateCaptureStartup(startErr == nil, startErr); err != nil {
		return finish(err)
	}
	select {
	case <-ready:
	case processErr := <-waited:
		return finish(fmt.Errorf("packet capture exited before readiness: %w: %s", processErr, <-stderrFinished))
	case <-time.After(5 * time.Second):
		_ = command.Process.Signal(os.Interrupt)
		processErr := <-waited
		return finish(fmt.Errorf("packet capture did not report readiness: %w: %s", processErr, <-stderrFinished))
	}
	// tcpdump reports that the PACKET socket is active before the bounded
	// capture loop begins. Keep the tracer gate closed for one short settling
	// interval so the first synthetic packet cannot race that transition.
	time.Sleep(250 * time.Millisecond)
	if err := writeToolingReady(evidenceDirectory, runID, result.Role); err != nil {
		_ = command.Process.Signal(os.Interrupt)
		<-waited
		return finish(err)
	}
	var captureStderr string
	select {
	case err := <-waited:
		captureStderr = <-stderrFinished
		if err != nil {
			return finish(fmt.Errorf("packet capture exit: %w: %s", err, captureStderr))
		}
	case <-time.After(10 * time.Second):
		_ = command.Process.Signal(os.Interrupt)
		processErr := <-waited
		captureStderr = <-stderrFinished
		return finish(fmt.Errorf("packet capture did not reach its fixed packet bound: %w: %s", processErr, captureStderr))
	}
	info, err := os.Stat(capturePath)
	if err != nil {
		return finish(err)
	}
	decoded, err := exec.Command("/usr/bin/tcpdump", "-Z", "root", "-A", "-nn", "-r", capturePath, "tcp", "port", toolingTCPPort).CombinedOutput()
	if err != nil {
		return finish(fmt.Errorf("decode bounded packet capture: %w: %s", err, strings.TrimSpace(string(decoded))))
	}
	marker := toolingMarkerPrefix + runID
	if err := validateCaptureEvidence(info.Size(), string(decoded), marker); err != nil {
		return finish(fmt.Errorf("%w: %s", err, captureStderr))
	}
	digest, err := hashRegularFile(capturePath)
	if err != nil {
		return finish(err)
	}
	result.CaptureSHA256 = digest
	result.CaptureBytes = info.Size()
	result.CaptureTracer = true
	if err := removeRawCapture(capturePath, filepath.Dir(capturePath)); err != nil {
		return finish(err)
	}
	result.RawCaptureRemoved = true
	return finish(nil)
}
