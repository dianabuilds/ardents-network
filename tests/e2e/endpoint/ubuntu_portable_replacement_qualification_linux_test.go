//go:build linux && endpoint_replacement_qualification

package endpoint_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestUbuntuPortableReplacementQualification proves Endpoint replacement against the same
// real unprivileged user-systemd shape selected for portable Endpoint. Unlike the ordinary
// process test, it does not replace systemctl with a recording fixture.
func TestUbuntuPortableReplacementQualification(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("Endpoint replacement qualification requires an unprivileged user session")
	}
	if runtimeDirectory := os.Getenv("XDG_RUNTIME_DIR"); !filepath.IsAbs(runtimeDirectory) {
		t.Fatal("Endpoint replacement qualification requires an absolute XDG_RUNTIME_DIR from a user session")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	stateHome := filepath.Join(home, ".local", "state", "ardents")
	qualificationRoots := []string{
		filepath.Join(home, ".config", "ardents"),
		stateHome,
		filepath.Join(home, ".cache", "ardents"),
		filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "ardents"),
	}
	for _, root := range qualificationRoots {
		requireAbsentQualificationRoot(t, root)
	}
	t.Cleanup(func() {
		for _, root := range qualificationRoots {
			removeQualificationRoot(t, root)
		}
	})
	if output, err := userSystemctl(t, "show-environment"); err != nil {
		t.Fatalf("Endpoint replacement qualification requires a reachable systemd --user manager: %v\n%s", err, output)
	}
	lingerBefore := userLinger(t)
	if lingerBefore != "no" {
		t.Fatalf("Endpoint replacement qualification requires linger=no before the unit: %q", lingerBefore)
	}

	command := buildArdents(t)
	bundle, enrolled, _, keys, rootBytes := enrolledRuntimeBundleWithKeys(t, command)
	manifestPin := enrolledRuntimeManifestPin(t, bundle)
	if err := os.Chmod(enrolled, 0o600); err != nil {
		t.Fatal(err)
	}
	verifyExternallyBeforeExecution(t, bundle, manifestPin)
	if err := os.Chmod(enrolled, 0o700); err != nil {
		t.Fatal(err)
	}
	unitName := "ardents-endpoint.service"
	unitPath := filepath.Join(home, ".config", "systemd", "user", unitName)
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o700); err != nil {
		t.Fatal(err)
	}
	unit, err := exec.Command(enrolled, "endpoint", "user-unit", bundle, manifestPin).Output()
	if err != nil {
		t.Fatalf("render participant unit: %v", err)
	}
	if err := os.WriteFile(unitPath, unit, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupQualificationUnit(t, unitName, unitPath) })
	if output, err := userSystemctl(t, "daemon-reload"); err != nil {
		t.Fatalf("reload participant unit: %v\n%s", err, output)
	}
	if output, err := userSystemctl(t, "enable", "--now", unitName); err != nil {
		t.Fatalf("enable and start participant unit: %v\n%s", err, output)
	}
	runtimeSocket := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "ardents", "endpoint.sock")
	waitForPortableReady(t, unitName, runtimeSocket)
	assertUserJournalContains(t, unitName, "\"state\":\"starting\"", "\"kind\":\"release-decision\"", "\"state\":\"ready\"")

	v1, err := os.ReadFile(enrolled)
	if err != nil {
		t.Fatal(err)
	}
	v2 := append(append([]byte(nil), v1...), 0)
	platform := runtime.GOOS + "-" + runtime.GOARCH
	targetPath := "ardents/" + platform + "/endpoint"
	metadata := replacementMetadata(t, v2, targetPath, platform, time.Now().UTC().Truncate(time.Second), keys)
	metadata["2.root.json"] = replacementRoot(t, rootBytes, keys)
	candidate := replacementBundle(t, v2, rootBytes, metadata, targetPath, platform)
	output, err := exec.Command(enrolled, "endpoint", "replace", candidate).CombinedOutput()
	if err != nil {
		t.Fatalf("replace through native systemd --user: %v\n%s", err, output)
	}
	var result struct{ Kind, State string }
	if err := json.Unmarshal(output, &result); err != nil || result.Kind != "endpoint-replacement" || result.State != "committed-restart-permitted" {
		t.Fatalf("native replacement result = %q / %+v / %v", output, result, err)
	}
	waitForPortableReady(t, unitName, runtimeSocket)
	if got, readErr := os.ReadFile(enrolled); readErr != nil || !bytes.Equal(got, v2) {
		t.Fatalf("native replacement program = %d bytes / %v", len(got), readErr)
	}
	if output, err := userSystemctl(t, "stop", unitName); err != nil {
		t.Fatalf("stop native replaced unit: %v\n%s", err, output)
	}
	assertPortableStopped(t, unitName, runtimeSocket)
	if output, err := userSystemctl(t, "start", unitName); err != nil {
		t.Fatalf("restart native replaced unit: %v\n%s", err, output)
	}
	waitForPortableReady(t, unitName, runtimeSocket)
	if lingerAfter := userLinger(t); lingerAfter != lingerBefore {
		t.Fatalf("participant replacement changed linger: before=%q after=%q", lingerBefore, lingerAfter)
	}
	if _, err := os.Stat(filepath.Join(stateHome, "vault")); err != nil {
		t.Fatalf("native replacement removed the protected Vault root: %v", err)
	}
	assertRetainedPortableState(t, stateHome)
	if output, err := userSystemctl(t, "stop", unitName); err != nil {
		t.Fatalf("final native replaced stop: %v\n%s", err, output)
	}
	assertPortableStopped(t, unitName, runtimeSocket)
}

// TestUbuntuPortableReplacementRollbackQualification proves that a real
// user-systemd session does not restart a valid-but-broken candidate and that
// only the retained predecessor, with a fresh Release authorization, can
// restore it. It intentionally uses a shell executable for v2: its bytes are
// authenticated, but its bounded replacement self-test must fail.
func TestUbuntuPortableReplacementRollbackQualification(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("Endpoint replacement qualification requires an unprivileged user session")
	}
	if runtimeDirectory := os.Getenv("XDG_RUNTIME_DIR"); !filepath.IsAbs(runtimeDirectory) {
		t.Fatal("Endpoint replacement qualification requires an absolute XDG_RUNTIME_DIR from a user session")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	stateHome := filepath.Join(home, ".local", "state", "ardents")
	qualificationRoots := []string{
		filepath.Join(home, ".config", "ardents"), stateHome,
		filepath.Join(home, ".cache", "ardents"), filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "ardents"),
	}
	for _, root := range qualificationRoots {
		requireAbsentQualificationRoot(t, root)
	}
	t.Cleanup(func() {
		for _, root := range qualificationRoots {
			removeQualificationRoot(t, root)
		}
	})
	if output, err := userSystemctl(t, "show-environment"); err != nil {
		t.Fatalf("Endpoint replacement qualification requires a reachable systemd --user manager: %v\n%s", err, output)
	}
	lingerBefore := userLinger(t)
	if lingerBefore != "no" {
		t.Fatalf("Endpoint replacement qualification requires linger=no before the unit: %q", lingerBefore)
	}

	command := buildArdents(t)
	bundle, enrolled, _, keys, rootBytes := enrolledRuntimeBundleWithKeys(t, command)
	manifestPin := enrolledRuntimeManifestPin(t, bundle)
	if err := os.Chmod(enrolled, 0o600); err != nil {
		t.Fatal(err)
	}
	verifyExternallyBeforeExecution(t, bundle, manifestPin)
	if err := os.Chmod(enrolled, 0o700); err != nil {
		t.Fatal(err)
	}
	unitName := "ardents-endpoint.service"
	unitPath := filepath.Join(home, ".config", "systemd", "user", unitName)
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o700); err != nil {
		t.Fatal(err)
	}
	unit, err := exec.Command(enrolled, "endpoint", "user-unit", bundle, manifestPin).Output()
	if err != nil {
		t.Fatalf("render participant unit: %v", err)
	}
	if err := os.WriteFile(unitPath, unit, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupQualificationUnit(t, unitName, unitPath) })
	if output, err := userSystemctl(t, "daemon-reload"); err != nil {
		t.Fatalf("reload participant unit: %v\n%s", err, output)
	}
	if output, err := userSystemctl(t, "enable", "--now", unitName); err != nil {
		t.Fatalf("enable and start participant unit: %v\n%s", err, output)
	}
	runtimeSocket := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "ardents", "endpoint.sock")
	waitForPortableReady(t, unitName, runtimeSocket)
	assertUserJournalContains(t, unitName, "\"state\":\"starting\"", "\"kind\":\"release-decision\"", "\"state\":\"ready\"")

	original, err := os.ReadFile(enrolled)
	if err != nil {
		t.Fatal(err)
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	targetPath := "ardents/" + platform + "/endpoint"
	failedCandidate := []byte("#!/bin/sh\nexit 1\n")
	candidateMetadata := replacementMetadataVersion(t, failedCandidate, targetPath, platform, time.Now().UTC().Truncate(time.Second), keys, 2, 2)
	candidateMetadata["2.root.json"] = replacementRootVersion(t, rootBytes, keys, 2)
	candidate := replacementBundle(t, failedCandidate, rootBytes, candidateMetadata, targetPath, platform)
	output, err := exec.Command(enrolled, "endpoint", "replace", candidate).CombinedOutput()
	if err == nil {
		t.Fatalf("native replacement unexpectedly accepted failed candidate:\n%s", output)
	}
	var failed struct {
		Kind            string `json:"Kind"`
		State           string `json:"State"`
		RecoveryProgram string `json:"recovery_program"`
	}
	firstLine := strings.SplitN(string(output), "\n", 2)[0]
	if err := json.Unmarshal([]byte(firstLine), &failed); err != nil || failed.Kind != "endpoint-replacement" ||
		failed.State != "rollback-authorization-required" || failed.RecoveryProgram == "" {
		t.Fatalf("failed native replacement result = %q / %+v / %v", output, failed, err)
	}
	if output, err := userSystemctl(t, "is-active", unitName); err == nil || strings.TrimSpace(string(output)) != "inactive" {
		t.Fatalf("failed candidate restarted native unit: %q / %v", output, err)
	}

	rollbackMetadata := replacementMetadataVersion(t, original, targetPath, platform, time.Now().UTC().Truncate(time.Second), keys, 3, 3)
	rollbackRoot := replacementRootVersion(t, rootBytes, keys, 2)
	rollbackMetadata["3.root.json"] = replacementRootVersion(t, rootBytes, keys, 3)
	rollbackBundle := replacementBundle(t, original, rollbackRoot, rollbackMetadata, targetPath, platform)
	output, err = exec.Command(failed.RecoveryProgram, "endpoint", "rollback", rollbackBundle).CombinedOutput()
	if err != nil {
		t.Fatalf("native retained-program rollback: %v\n%s", err, output)
	}
	var restored struct{ Kind, State string }
	if err := json.Unmarshal(output, &restored); err != nil || restored.Kind != "endpoint-rollback" || restored.State != "rollback-committed-restart-permitted" {
		t.Fatalf("native rollback result = %q / %+v / %v", output, restored, err)
	}
	waitForPortableReady(t, unitName, runtimeSocket)
	if got, readErr := os.ReadFile(enrolled); readErr != nil || !bytes.Equal(got, original) {
		t.Fatalf("native rollback program = %d bytes / %v", len(got), readErr)
	}
	if lingerAfter := userLinger(t); lingerAfter != lingerBefore {
		t.Fatalf("participant rollback changed linger: before=%q after=%q", lingerBefore, lingerAfter)
	}
	if _, err := os.Stat(filepath.Join(stateHome, "vault")); err != nil {
		t.Fatalf("native rollback removed the protected Vault root: %v", err)
	}
	assertRetainedPortableState(t, stateHome)
	if output, err := userSystemctl(t, "stop", unitName); err != nil {
		t.Fatalf("final native rollback stop: %v\n%s", err, output)
	}
	assertPortableStopped(t, unitName, runtimeSocket)
}
