//go:build linux && endpoint_launch_native

package endpointinstalled_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// TestNativeEndpointPlatformLedLaunchRefusals checks substitutions against an admitted platform delivery.
func TestNativeEndpointPlatformLedLaunchRefusals(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("endpoint platform-led launch qualification requires a root driver")
	}
	spec, _, _ := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
	stand := endpointfixture.ProvisionLaunchStand(t, buildArdentsNext(t), 1000, spec, false, nil)
	if out, err := exec.CommandContext(t.Context(), stand.Binary, "platform", "check", stand.Manifest, stand.Pin).CombinedOutput(); err != nil {
		t.Fatalf("genuine platform check refused before applying substitutions: %v: %s", err, out)
	}
	credential := filepath.Join(stand.EtcDir, "client.key")
	for _, fault := range []string{"substituted-program", "substituted-unit", "wrong-account", "foreign-pin", "substituted-credential"} {
		t.Run(fault, func(t *testing.T) {
			mutateLaunchFault(t, stand.Program, stand.Manifest, credential, fault)
			out, err := exec.CommandContext(t.Context(), stand.Binary, "platform", "check", stand.Manifest, stand.Pin).CombinedOutput()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() <= 0 {
				t.Fatalf("expected platform check to refuse %s: %v: %s", fault, err, out)
			}
		})
	}
}

func mutateLaunchFault(t *testing.T, program, manifest, credential, fault string) {
	t.Helper()
	switch fault {
	case "substituted-program":
		original, err := os.ReadFile(program)
		if err != nil {
			t.Fatal(err)
		}
		changed := bytes.Clone(original)
		changed[len(changed)-1] ^= 1
		if err := os.WriteFile(program, changed, 0555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { restoreLaunchFile(t, program, original, 0555) })
	case "substituted-unit":
		original, err := os.ReadFile("/etc/systemd/system/ardents-endpoint.service")
		if err != nil {
			t.Fatal(err)
		}
		changed := append(bytes.Clone(original), []byte("# foreign drop-in comment\n")...)
		if err := os.WriteFile("/etc/systemd/system/ardents-endpoint.service", changed, 0644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			restoreLaunchFile(t, "/etc/systemd/system/ardents-endpoint.service", original, 0644)
			reloadLaunchUnit(t)
		})
		reloadLaunchUnit(t)

	case "wrong-account":
		if err := exec.Command("usermod", "-s", "/bin/bash", "ardents-endpoint").Run(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if out, err := exec.Command("usermod", "-s", "/usr/sbin/nologin", "ardents-endpoint").CombinedOutput(); err != nil {
				t.Errorf("restore Endpoint account: %v: %s", err, out)
			}
		})
	case "foreign-pin":
		original, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatal(err)
		}
		var launch struct {
			ConfigSHA256 string `json:"config_sha256"`
		}
		if err := json.Unmarshal(original, &launch); err != nil {
			t.Fatal(err)
		}
		changed := bytes.Replace(original, []byte(launch.ConfigSHA256), []byte(strings.Repeat("0", 64)), 1)
		if err := os.WriteFile(manifest, changed, 0644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { restoreLaunchFile(t, manifest, original, 0644) })
	case "substituted-credential":
		original, err := os.ReadFile(credential)
		if err != nil {
			t.Fatal(err)
		}
		changed := bytes.Clone(original)
		changed[len(changed)-1] ^= 1
		if err := os.WriteFile(credential, changed, 0600); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { restoreLaunchFile(t, credential, original, 0600) })
	default:
		t.Fatal("unknown fault", fault)

	}
}

func restoreLaunchFile(t testing.TB, path string, original []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, original, mode); err != nil {
		t.Errorf("restore %s: %v", path, err)
	}
}

func reloadLaunchUnit(t testing.TB) {
	t.Helper()
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		t.Fatalf("reload Endpoint unit: %v: %s", err, out)
	}
}
