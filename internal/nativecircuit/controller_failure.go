package nativecircuit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

func nativeRunChecks(fault string) map[string]bool {
	if fault == "rendezvous-process" {
		return map[string]bool{
			"verified_images": false, "fixed_topology": false, "bounded_capabilities": false,
			"rendezvous_process_killed": false, "explicit_terminal_result": false,
			"verified_prefix_bounded": false, "unauthenticated_bytes_absent": false,
			"failure_within_15_seconds": false, "cleanup_complete": false,
		}
	}
	return map[string]bool{
		"verified_images": false, "fixed_topology": false, "bounded_capabilities": false, "shaping_applied": false,
		"capture_complete": false, "forbidden_sentinels_absent": false,
		"exact_instance_authenticated": false, "application_stream_verified": false,
		"role_views": false, "raw_capture_removed": false, "cleanup_complete": false,
	}
}

func runNativeRendezvousFailure(ctx context.Context, layout nativeRunLayout, fixture nativeFixture, project string, environment []string, summary *nativeRunSummary) error {
	ready := filepath.Join(fixture.roleEvidence["user"], "stream-ready.json")
	if err := waitNativeReady(ctx, fixture, []string{ready}, 30*time.Second); err != nil {
		return err
	}
	started := time.Now()
	if _, err := nativeCompose(ctx, layout, project, environment, "kill", "--signal", "SIGKILL", "rendezvous"); err != nil {
		return err
	}
	summary.Checks["rendezvous_process_killed"] = true
	if err := writeControlMarker(fixture.controlDirectory, "stream-continue"); err != nil {
		return err
	}
	paths := []string{
		filepath.Join(fixture.roleEvidence["user"], "result.json"),
		filepath.Join(fixture.roleEvidence["service"], "result.json"),
	}
	if err := waitNativeReady(ctx, fixture, paths, 15*time.Second); err != nil {
		return err
	}
	summary.Checks["failure_within_15_seconds"] = time.Since(started) <= 15*time.Second
	user, err := readFailureRoleResult(paths[0])
	if err != nil {
		return err
	}
	service, err := readFailureRoleResult(paths[1])
	if err != nil {
		return err
	}
	if err := retainFailureRoleResults(layout.evidenceDir, user, service); err != nil {
		return err
	}
	summary.Checks["explicit_terminal_result"] = user.Status == "failed" && service.Status == "failed" && user.TerminalResult == "explicit_failure" && service.TerminalResult == "explicit_failure"
	summary.Checks["verified_prefix_bounded"] = user.ApplicationBytes == maximumApplicationPayload && service.ApplicationBytes == maximumApplicationPayload
	summary.Checks["unauthenticated_bytes_absent"] = !user.ApplicationBytesVerified && !service.ApplicationBytesVerified && user.ApplicationBytes == service.ApplicationBytes
	for name, passed := range summary.Checks {
		if name != "cleanup_complete" && !passed {
			return errors.New("native Rendezvous process failure did not satisfy the fixed fail-closed contract")
		}
	}
	_ = writeControlMarker(fixture.controlDirectory, "stop")
	_ = writeControlMarker(fixture.controlDirectory, "capture-cleanup")
	summary.Status = "passed"
	summary.Verdict = "failure_smoke_passed"
	return nil
}

func retainFailureRoleResults(evidenceDir string, user, service collectedRoleResult) error {
	directory := filepath.Join(evidenceDir, "native-roles")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return err
	}
	if err := writeRoleJSON(filepath.Join(directory, "user.json"), user); err != nil {
		return err
	}
	return writeRoleJSON(filepath.Join(directory, "service.json"), service)
}

func readFailureRoleResult(path string) (collectedRoleResult, error) {
	data, err := readBoundedEvidence(path)
	if err != nil {
		return collectedRoleResult{}, err
	}
	var result collectedRoleResult
	if err := json.Unmarshal(data, &result); err != nil {
		return collectedRoleResult{}, err
	}
	return result, nil
}
