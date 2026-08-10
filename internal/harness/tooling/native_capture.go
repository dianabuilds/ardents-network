package tooling

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type nativeCaptureResult struct {
	SchemaVersion         string                         `json:"schema_version"`
	RunID                 string                         `json:"run_id"`
	Role                  string                         `json:"role"`
	Status                string                         `json:"status"`
	EffectiveCapabilities string                         `json:"effective_capabilities"`
	ToolLockSHA256        string                         `json:"tool_lock_sha256"`
	Tools                 map[string]observedTool        `json:"tools"`
	Links                 map[string]nativeCaptureRecord `json:"links"`
	RawCaptureRemoved     bool                           `json:"raw_capture_removed"`
	Failure               string                         `json:"failure,omitempty"`
}

type nativeCaptureRecord struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	Packet bool   `json:"packet_observed"`
}

type nativeCaptureProcess struct {
	link    nativeCaptureLink
	path    string
	command *exec.Cmd
	waited  chan error
	ready   chan struct{}
	stderr  chan string
}

func runNativeCapture(config nativeToolConfig, evidenceDirectory, captureDirectory string) (runErr error) {
	result := nativeCaptureResult{SchemaVersion: nativeToolRoleSchema, RunID: config.RunID, Role: config.Role, Status: "failed", Links: make(map[string]nativeCaptureRecord)}
	paths := make([]string, 0, len(config.Links))
	defer func() {
		var cleanupErr error
		for _, path := range paths {
			cleanupErr = errors.Join(cleanupErr, removeRawCapture(path, captureDirectory))
		}
		result.RawCaptureRemoved = cleanupErr == nil
		runErr = errors.Join(runErr, cleanupErr)
		if runErr == nil {
			result.Status = "passed"
		} else {
			result.Failure = runErr.Error()
		}
		runErr = errors.Join(runErr, writeBoundedJSON(filepath.Join(evidenceDirectory, "result.json"), result))
	}()
	capabilities, err := effectiveCapabilities()
	result.EffectiveCapabilities = capabilities
	if err != nil || !hasOnlyEffectiveCapability(capabilities, capabilityNetRaw) {
		return errors.Join(err, errors.New("native capture requires exactly NET_RAW"))
	}
	identity, tools, err := observeTools(toolingLockPath, "tcpdump", "libpcap")
	if err != nil {
		return err
	}
	result.ToolLockSHA256, result.Tools = identity.LockSHA256, tools
	processes := make([]nativeCaptureProcess, 0, len(config.Links))
	for _, link := range config.Links {
		path := filepath.Join(captureDirectory, link.Name+".pcap")
		paths = append(paths, path)
		addresses, err := resolveNativeCapturePeer(link.Peer, 10*time.Second)
		if err != nil {
			stopNativeCaptures(processes)
			return err
		}
		process, err := startNativeCapture(link, path, addresses)
		if err != nil {
			stopNativeCaptures(processes)
			return err
		}
		processes = append(processes, process)
	}
	for _, process := range processes {
		select {
		case <-process.ready:
		case err := <-process.waited:
			stopNativeCaptures(processes)
			return fmt.Errorf("capture %s exited before readiness: %w: %s", process.link.Name, err, <-process.stderr)
		case <-time.After(5 * time.Second):
			stopNativeCaptures(processes)
			return errors.New("native capture did not report readiness")
		}
	}
	if err := writeToolingReady(evidenceDirectory, config.RunID, config.Role); err != nil {
		stopNativeCaptures(processes)
		return err
	}
	if err := waitForControlFile(filepath.Join("/control", "stop"), 30*time.Minute); err != nil {
		stopNativeCaptures(processes)
		return err
	}
	stopNativeCaptures(processes)
	for _, process := range processes {
		if err := <-process.waited; err != nil {
			return fmt.Errorf("capture %s stop: %w: %s", process.link.Name, err, <-process.stderr)
		}
		<-process.stderr
		record, err := inspectNativeCapture(process.path)
		if err != nil {
			return err
		}
		result.Links[process.link.Name] = record
	}
	if err := writeBoundedJSON(filepath.Join(evidenceDirectory, "capture-ready.json"), result.Links); err != nil {
		return err
	}
	if err := waitForControlFile(filepath.Join("/control", "capture-cleanup"), 30*time.Minute); err != nil {
		return err
	}
	for _, path := range paths {
		if err := removeRawCapture(path, captureDirectory); err != nil {
			return err
		}
	}
	paths = nil
	result.RawCaptureRemoved = true
	return nil
}

func startNativeCapture(link nativeCaptureLink, path string, addresses []string) (nativeCaptureProcess, error) {
	_ = os.Remove(path)
	arguments := []string{"-Z", "root", "-i", "any", "-n", "-U", "-s", "256", "-B", "1024", "-w", path}
	arguments = append(arguments, nativeCaptureFilter(addresses)...)
	command := exec.Command("/usr/bin/tcpdump", arguments...)
	stderrPipe, err := command.StderrPipe()
	process := nativeCaptureProcess{link: link, path: path, command: command, waited: make(chan error, 1), ready: make(chan struct{}, 1), stderr: make(chan string, 1)}
	if err != nil {
		return process, err
	}
	if err := command.Start(); err != nil {
		return process, err
	}
	go scanCaptureStderr(stderrPipe, process.ready, process.stderr)
	go func() { process.waited <- command.Wait() }()
	return process, nil
}

func resolveNativeCapturePeer(peer string, timeout time.Duration) ([]string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		addresses, err := net.LookupHost(peer)
		if err == nil && len(addresses) > 0 {
			return addresses, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("native capture peer %s did not resolve before readiness", peer)
}

func nativeCaptureFilter(addresses []string) []string {
	filter := make([]string, 0, len(addresses)*3)
	for index, address := range addresses {
		if index > 0 {
			filter = append(filter, "or")
		}
		filter = append(filter, "host", address)
	}
	return filter
}

func stopNativeCaptures(processes []nativeCaptureProcess) {
	for _, process := range processes {
		if process.command.Process != nil {
			_ = process.command.Process.Signal(os.Interrupt)
		}
	}
}

func inspectNativeCapture(path string) (nativeCaptureRecord, error) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= 24 {
		return nativeCaptureRecord{}, errors.New("native link capture is empty")
	}
	output, err := exec.Command("/usr/bin/tcpdump", "-Z", "root", "-nn", "-r", path, "-c", "1").CombinedOutput()
	if err != nil {
		return nativeCaptureRecord{}, fmt.Errorf("verify native link capture: %w: %s", err, strings.TrimSpace(string(output)))
	}
	digest, err := hashRegularFile(path)
	if err != nil {
		return nativeCaptureRecord{}, err
	}
	return nativeCaptureRecord{SHA256: digest, Bytes: info.Size(), Packet: true}, nil
}
