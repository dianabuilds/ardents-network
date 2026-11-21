//go:build linux && text_worker_installed

package main

import (
	"bytes"

	"crypto/ed25519"

	"encoding/json"

	"os"
	"os/exec"

	"path/filepath"
	"runtime"

	"strings"

	"syscall"
	"testing"

	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The root driver runs outside the Endpoint unit. Its actual pinned program is
// the non-root Endpoint MainPID; only genuine Installation commands create the
// account, fixed resources, Root completion exchange and selected generation.
// All signing, Source and clock fixtures have one controller. This proves no
// independent custody, trusted time, private Service or whole-host qualification.

func installedSignedNetwork(t *testing.T, f *networkAdmissionFixture) installedStartupNetwork {
	t.Helper()
	epoch, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	var authority [32]byte
	copy(authority[:], f.spec.Authority.Public().(ed25519.PublicKey))
	return installedStartupNetwork{NetworkID: f.spec.NetworkID, Seed: f.spec.Seed, Authority: authority,
		Epoch: epoch.Epoch.Raw, Inputs: epoch.Epoch.Inputs, Materials: epoch.Epoch.Materials, Profile: epoch.Profile}
}

// This fixture input contains only public signed material and trust pins.
// Every participant still authenticates it through its own actual State;
// no live State, signing key, qualified operation or Instance crosses managers.
type installedStartupNetwork struct {
	NetworkID, Seed, Authority [32]byte
	Epoch, Profile             []byte
	Inputs, Materials          [][]byte
}

func installedStartupWrite(t *testing.T, path string, raw []byte, mode os.FileMode) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		t.Fatal(err)
	}
	n, writeErr := file.Write(raw)
	modeErr := file.Chmod(mode)
	syncErr := file.Sync()
	closeErr := file.Close()
	if n != len(raw) || writeErr != nil || modeErr != nil || syncErr != nil || closeErr != nil {
		t.Fatal("durable fixture write", writeErr, modeErr, syncErr, closeErr)
	}
	installedStartupSync(t, filepath.Dir(path))
}

func installedStartupSync(t *testing.T, path string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	syncErr, closeErr := file.Sync(), file.Close()
	if syncErr != nil || closeErr != nil {
		t.Fatal(syncErr, closeErr)
	}
}

func installedStartupCommand(t *testing.T, program string, credential *syscall.Credential, outcome string, args ...string) []byte {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), program, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential, Pdeathsig: syscall.SIGKILL}
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	runtime.LockOSThread()
	out, err := cmd.Output()
	runtime.UnlockOSThread()
	var response struct{ Outcome string }
	if err != nil || json.Unmarshal(out, &response) != nil || response.Outcome != outcome {
		t.Fatalf("actual %v: %v: %s; diagnostics: %s", args, err, out, diagnostic.String())
	}
	return out
}

func installedStartupIdentity(t *testing.T) [3]string {
	t.Helper()
	var result [3]string
	for i, property := range []string{"MainPID", "InvocationID", "ActiveState"} {
		out, err := exec.Command("systemctl", "show", "ardents-endpoint.service", "-p", property, "--value").CombinedOutput()
		if err != nil {
			t.Fatalf("actual %s: %v: %s", property, err, out)
		}
		result[i] = strings.TrimSpace(string(out))
	}
	return result
}
