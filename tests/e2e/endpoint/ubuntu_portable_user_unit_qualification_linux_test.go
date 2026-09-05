//go:build linux && (endpoint_portable_qualification || endpoint_replacement_qualification)

package endpoint_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUbuntuPortableUserUnitQualification(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("portable Endpoint qualification requires an unprivileged user session")
	}
	if runtimeDirectory := os.Getenv("XDG_RUNTIME_DIR"); !filepath.IsAbs(runtimeDirectory) {
		t.Fatal("portable Endpoint qualification requires an absolute XDG_RUNTIME_DIR from a user session")
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
		t.Fatalf("portable Endpoint qualification requires a reachable systemd --user manager: %v\n%s", err, output)
	}
	lingerBefore := userLinger(t)
	if lingerBefore != "no" {
		t.Fatalf("portable Endpoint qualification requires linger=no before the unit: %q", lingerBefore)
	}

	command := buildArdents(t)
	bundle, enrolledCommand, _ := enrolledRuntimeBundle(t, command)
	manifestPin := enrolledRuntimeManifestPin(t, bundle)
	if err := os.Chmod(enrolledCommand, 0o600); err != nil {
		t.Fatal(err)
	}
	verifyExternallyBeforeExecution(t, bundle, manifestPin)
	if err := os.Chmod(enrolledCommand, 0o700); err != nil {
		t.Fatal(err)
	}

	unitName := "ardents-endpoint-portable-ubuntu.service"
	unitPath := filepath.Join(home, ".config", "systemd", "user", unitName)
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o700); err != nil {
		t.Fatal(err)
	}
	unit, err := exec.Command(enrolledCommand, "endpoint", "user-unit", bundle, manifestPin).Output()
	if err != nil {
		t.Fatalf("render participant unit: %v", err)
	}
	if bytes.Contains(unit, []byte("User=")) || !bytes.Contains(unit, []byte("UMask=0077\nRestart=no\n")) {
		t.Fatalf("rendered user unit violates portable Endpoint profile:\n%s", unit)
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
	if output, err := userSystemctl(t, "stop", unitName); err != nil {
		t.Fatalf("stop first participant start: %v\n%s", err, output)
	}
	assertPortableStopped(t, unitName, runtimeSocket)
	assertRetainedPortableState(t, stateHome)

	if output, err := userSystemctl(t, "start", unitName); err != nil {
		t.Fatalf("restart participant unit: %v\n%s", err, output)
	}
	waitForPortableReady(t, unitName, runtimeSocket)
	if output, err := userSystemctl(t, "stop", unitName); err != nil {
		t.Fatalf("stop restarted participant unit: %v\n%s", err, output)
	}
	assertPortableStopped(t, unitName, runtimeSocket)
	assertRetainedPortableState(t, stateHome)

	if output, err := userSystemctl(t, "disable", unitName); err != nil {
		t.Fatalf("disable participant unit: %v\n%s", err, output)
	}
	if err := os.Remove(unitPath); err != nil {
		t.Fatal(err)
	}
	if output, err := userSystemctl(t, "daemon-reload"); err != nil {
		t.Fatalf("reload after unit removal: %v\n%s", err, output)
	}
	if err := os.RemoveAll(bundle); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(stateHome, "vault")); err != nil {
		t.Fatalf("deleting program bytes removed the protected Vault root: %v", err)
	}
	assertRetainedPortableState(t, stateHome)
	if lingerAfter := userLinger(t); lingerAfter != lingerBefore {
		t.Fatalf("participant unit changed linger: before=%q after=%q", lingerBefore, lingerAfter)
	}
}

func requireAbsentQualificationRoot(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Fatalf("portable Endpoint qualification requires a clean user account; occupied root %s", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("inspect required-empty qualification root %s: %v", path, err)
	}
}

func removeQualificationRoot(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Errorf("inspect qualification cleanup root %s: %v", path, err)
		return
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Errorf("refusing unexpected qualification cleanup root %s", path)
		return
	}
	if err := os.RemoveAll(path); err != nil {
		t.Errorf("remove qualification root %s: %v", path, err)
		return
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("qualification cleanup left root %s: %v", path, err)
	}
}

func cleanupQualificationUnit(t *testing.T, unitName, unitPath string) {
	t.Helper()
	_, statErr := os.Lstat(unitPath)
	if statErr == nil {
		if output, err := userSystemctl(t, "stop", unitName); err != nil {
			t.Errorf("cleanup stop %s: %v\\n%s", unitName, err, output)
		}
		if output, err := userSystemctl(t, "disable", unitName); err != nil {
			t.Errorf("cleanup disable %s: %v\\n%s", unitName, err, output)
		}
	} else if !os.IsNotExist(statErr) {
		t.Errorf("inspect qualification unit %s: %v", unitPath, statErr)
	}
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		t.Errorf("cleanup remove unit %s: %v", unitPath, err)
	}
	if _, err := os.Lstat(unitPath); !os.IsNotExist(err) {
		t.Errorf("cleanup retained unit %s: %v", unitPath, err)
	}
	if output, err := userSystemctl(t, "daemon-reload"); err != nil {
		t.Errorf("cleanup reload user manager: %v\\n%s", err, output)
	}
}

func verifyExternallyBeforeExecution(t *testing.T, bundle, manifestPin string) {
	t.Helper()
	digestCommand := exec.Command("sha256sum", "SHA256SUMS")
	digestCommand.Dir = bundle
	digest, err := digestCommand.Output()
	if err != nil {
		t.Fatalf("run external sha256sum: %v", err)
	}
	if actual := strings.Fields(string(digest)); len(actual) != 2 || actual[0] != manifestPin || actual[1] != "SHA256SUMS" {
		t.Fatalf("external manifest digest = %q, want %q", digest, manifestPin)
	}
	check := exec.Command("sha256sum", "--strict", "--check", "SHA256SUMS")
	check.Dir = bundle
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("external bundle digest check: %v\n%s", err, output)
	}
}

func userSystemctl(t *testing.T, arguments ...string) ([]byte, error) {
	t.Helper()
	return exec.Command("systemctl", append([]string{"--user"}, arguments...)...).CombinedOutput()
}

func userLinger(t *testing.T) string {
	t.Helper()
	output, err := exec.Command("loginctl", "show-user", fmt.Sprintf("%d", os.Geteuid()), "-p", "Linger", "--value").Output()
	if err != nil {
		t.Fatalf("inspect user linger: %v", err)
	}
	return strings.TrimSpace(string(output))
}

func waitForPortableReady(t *testing.T, unitName, runtimeSocket string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		if err := probePortableSocket(runtimeSocket); err == nil {
			return
		} else {
			last = err
		}
		time.Sleep(50 * time.Millisecond)
	}
	status, _ := userSystemctl(t, "status", unitName, "--no-pager")
	t.Fatalf("participant unit did not expose readiness at %s: %v\n%s", runtimeSocket, last, status)
}

func probePortableSocket(path string) error {
	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(time.Second)); err != nil {
		return err
	}
	if _, err := io.WriteString(connection, "probe\n"); err != nil {
		return err
	}
	response := make([]byte, len("ready\n"))
	if _, err := io.ReadFull(connection, response); err != nil {
		return err
	}
	if string(response) != "ready\n" {
		return errors.New("unexpected readiness response")
	}
	return nil
}

func assertUserJournalContains(t *testing.T, unitName string, fragments ...string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		output, err := userSystemctl(t, "status", unitName, "--no-pager")
		if err == nil {
			allPresent := true
			for _, fragment := range fragments {
				if !bytes.Contains(output, []byte(fragment)) {
					allPresent = false
					break
				}
			}
			if allPresent {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	output, _ := userSystemctl(t, "status", unitName, "--no-pager")
	t.Fatalf("participant journal lacks lifecycle observations %q:\n%s", fragments, output)
}

func assertPortableStopped(t *testing.T, unitName, runtimeSocket string) {
	t.Helper()
	if output, err := userSystemctl(t, "show", unitName, "-p", "ActiveState", "--value"); err != nil || strings.TrimSpace(string(output)) != "inactive" {
		t.Fatalf("participant unit did not stop: state=%q err=%v", output, err)
	}
	if _, err := os.Lstat(runtimeSocket); !os.IsNotExist(err) {
		t.Fatalf("live socket remains after participant stop: %v", err)
	}
}

func assertRetainedPortableState(t *testing.T, stateHome string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(stateHome, "floors", "release-decision", "current")); err != nil {
		t.Fatalf("release floor did not survive participant lifecycle: %v", err)
	}
	if info, err := os.Stat(stateHome); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("participant state root is not private: info=%v err=%v", info, err)
	}
}
