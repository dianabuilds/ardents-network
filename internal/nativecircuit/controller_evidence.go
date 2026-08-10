package nativecircuit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type collectedRoleResult struct {
	Status                    string   `json:"status"`
	TerminalResult            string   `json:"terminal_result"`
	TLSVersion                string   `json:"tls_version"`
	ApplicationBytesVerified  bool     `json:"application_bytes_verified"`
	ApplicationBytes          int      `json:"application_bytes"`
	QueueHighWaterBytes       int      `json:"queue_high_water_bytes"`
	StreamElapsedMilliseconds int64    `json:"stream_elapsed_milliseconds"`
	ObservedFields            []string `json:"observed_fields"`
}

type collectedToolResult struct {
	Status                string            `json:"status"`
	EffectiveCapabilities string            `json:"effective_capabilities"`
	Qdisc                 map[string]string `json:"qdisc"`
	Links                 map[string]struct {
		Packet    bool   `json:"packet_observed"`
		Bytes     int64  `json:"bytes"`
		WireBytes uint64 `json:"wire_bytes"`
	} `json:"links"`
	RawCaptureRemoved bool `json:"raw_capture_removed"`
}

func collectNativeEvidence(fixture nativeFixture, retained string, summary *nativeRunSummary) error {
	roleTarget := filepath.Join(retained, "native-roles")
	toolTarget := filepath.Join(retained, "native-tools")
	for _, directory := range []string{roleTarget, toolTarget} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return err
		}
	}
	roleViews := true
	for role, directory := range fixture.roleEvidence {
		data, err := readBoundedEvidence(filepath.Join(directory, "result.json"))
		if err != nil {
			return err
		}
		var result collectedRoleResult
		if err := json.Unmarshal(data, &result); err != nil || result.Status != "passed" {
			return fmt.Errorf("native role %s did not pass", role)
		}
		roleViews = roleViews && len(result.ObservedFields) > 0
		if role == "user" {
			summary.Checks["exact_instance_authenticated"] = result.TLSVersion == "TLS1.3"
			summary.Checks["application_stream_verified"] = result.ApplicationBytesVerified
		}
		if err := os.WriteFile(filepath.Join(roleTarget, role+".json"), data, 0o600); err != nil {
			return err
		}
	}
	shaping, capture, rawRemoved := true, true, true
	for role, directory := range fixture.toolEvidence {
		data, err := readBoundedEvidence(filepath.Join(directory, "result.json"))
		if err != nil {
			return err
		}
		var result collectedToolResult
		if err := json.Unmarshal(data, &result); err != nil || result.Status != "passed" {
			return fmt.Errorf("native tool role %s did not pass", role)
		}
		if strings.HasPrefix(role, "shape-") {
			shaping = shaping && result.EffectiveCapabilities == "0000000000001000" && len(result.Qdisc) > 0
			for _, state := range result.Qdisc {
				shaping = shaping && strings.Contains(strings.ToLower(state), "rate 100mbit") && strings.Contains(strings.ToLower(state), "limit 1000")
			}
			if role == "shape-user" || role == "shape-service" {
				for _, state := range result.Qdisc {
					shaping = shaping && strings.Contains(strings.ToLower(state), "delay 40ms")
				}
			}
		} else {
			capture = capture && result.EffectiveCapabilities == "0000000000002000" && len(result.Links) > 0
			for _, link := range result.Links {
				capture = capture && link.Packet && link.Bytes > 24
			}
			rawRemoved = rawRemoved && result.RawCaptureRemoved
		}
		if err := os.WriteFile(filepath.Join(toolTarget, role+".json"), data, 0o600); err != nil {
			return err
		}
	}
	summary.Checks["role_views"] = roleViews
	summary.Checks["shaping_applied"] = shaping
	summary.Checks["capture_complete"] = capture
	summary.Checks["raw_capture_removed"] = rawRemoved
	for name, passed := range summary.Checks {
		if name != "cleanup_complete" && !passed {
			return fmt.Errorf("native evidence check %s did not pass", name)
		}
	}
	return nil
}

func inspectForbiddenSentinels(directory string, sentinels [][]byte, expectedCaptures int) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	captures := 0
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pcap") {
			return errors.New("raw capture directory contains an unexpected artifact")
		}
		info, err := entry.Info()
		if err != nil || info.Size() <= 24 || info.Size() > 64*1024*1024 {
			return errors.New("native link capture is empty or exceeds 64 MiB")
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return err
		}
		for _, sentinel := range sentinels {
			if len(sentinel) > 0 && bytes.Contains(data, sentinel) {
				return fmt.Errorf("forbidden sentinel appears in cleartext capture %s", entry.Name())
			}
		}
		captures++
		total += info.Size()
	}
	if captures != expectedCaptures || total > 2*1024*1024*1024 {
		return fmt.Errorf("native capture set is not exact: files=%d bytes=%d", captures, total)
	}
	return nil
}

func finishNativeRun(layout nativeRunLayout, fixture nativeFixture, project string, environment []string, summary *nativeRunSummary, runErr error) error {
	_ = writeControlMarker(fixture.controlDirectory, "stop")
	_ = writeControlMarker(fixture.controlDirectory, "capture-cleanup")
	cleanupContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if runErr != nil {
		captureNativeFailureLog(cleanupContext, layout, project, environment)
	}
	_, downErr := nativeCompose(cleanupContext, layout, project, environment, "down", "--volumes", "--remove-orphans", "--timeout", "5")
	if nativeResourcesRemain(cleanupContext, project) {
		downErr = errors.Join(downErr, errors.New("native Compose resources remain after cleanup"))
	}
	removeErr := removeNativeRunDirectory(layout, fixture)
	if downErr == nil && removeErr == nil {
		summary.Checks["cleanup_complete"] = true
	}
	cleanupErr := errors.Join(downErr, removeErr)
	if runErr != nil || cleanupErr != nil {
		summary.Status = "failed"
		summary.Verdict = "invalid"
		summary.Failure = sanitizeNativeFailure(layout, errors.Join(runErr, cleanupErr).Error())
	}
	evidenceErr := writeRoleJSON(filepath.Join(layout.evidenceDir, "native-run.json"), summary)
	return errors.Join(runErr, cleanupErr, evidenceErr)
}

func captureNativeFailureLog(ctx context.Context, layout nativeRunLayout, project string, environment []string) {
	output, err := nativeCompose(ctx, layout, project, environment, "logs", "--no-color", "--timestamps")
	if err != nil || len(output) == 0 {
		return
	}
	const maximumFailureLog = 8 * 1024 * 1024
	if len(output) > maximumFailureLog {
		output = output[len(output)-maximumFailureLog:]
	}
	redacted := sanitizeNativeFailure(layout, string(output))
	_ = os.WriteFile(filepath.Join(layout.evidenceDir, "native-compose.log"), []byte(redacted), 0o600)
}

func writeControlMarker(directory, name string) error {
	path := filepath.Join(directory, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return file.Close()
}

func removeNativeRunDirectory(layout nativeRunLayout, fixture nativeFixture) error {
	if _, err := openNativeRunLayout(layout.identity, false, false); err != nil {
		return err
	}
	target := layout.runDirectory
	if layout.shared {
		target = fixture.root
	}
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("native run path is not an owned directory")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		return errors.New("native run directory remains after cleanup")
	}
	return nil
}

func nativeResourcesRemain(ctx context.Context, project string) bool {
	queries := [][]string{{"ps", "--all", "--quiet", "--filter", "label=com.docker.compose.project=" + project}, {"network", "ls", "--quiet", "--filter", "label=com.docker.compose.project=" + project}, {"volume", "ls", "--quiet", "--filter", "label=com.docker.compose.project=" + project}}
	for _, arguments := range queries {
		output, err := exec.CommandContext(ctx, "docker", arguments...).Output()
		if err != nil || strings.TrimSpace(string(output)) != "" {
			return true
		}
	}
	return false
}

func readBoundedEvidence(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= 0 || info.Size() > 32*1024*1024 {
		return nil, errors.New("native evidence is missing or oversized")
	}
	return os.ReadFile(path)
}
