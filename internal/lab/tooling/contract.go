package tooling

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	capabilityNetAdmin = 12
	capabilityNetRaw   = 13
)

var requiredToolingChecks = []string{
	"tool_identity",
	"shaping_alpha",
	"shaping_beta",
	"capture_started",
	"capture_nonempty",
	"capture_tracer",
	"peer_set",
	"image_receipt",
	"isolation",
	"cleanup_complete",
	"raw_capture_removed",
}

type externalCommand func(name string, arguments ...string) ([]byte, error)

func hasEffectiveCapability(capabilityHex string, bit uint) bool {
	value := new(big.Int)
	if _, ok := value.SetString(strings.TrimSpace(capabilityHex), 16); !ok {
		return false
	}
	return value.Bit(int(bit)) == 1
}

func applyAndObserveShaping(run externalCommand, networkInterface string) (string, error) {
	if networkInterface == "" || strings.ContainsAny(networkInterface, "/\\ \t\r\n") {
		return "", errors.New("invalid shaping interface")
	}
	output, err := run("/usr/sbin/tc", "qdisc", "replace", "dev", networkInterface, "root", "netem", "delay", "40ms", "rate", "100mbit", "limit", "1000")
	if err != nil {
		return "", fmt.Errorf("apply tc netem: %w: %s", err, strings.TrimSpace(string(output)))
	}
	observed, err := run("/usr/sbin/tc", "-details", "qdisc", "show", "dev", networkInterface)
	if err != nil {
		return "", fmt.Errorf("observe tc netem: %w: %s", err, strings.TrimSpace(string(observed)))
	}
	state := string(observed)
	if !fixedQdiscState(state) {
		return "", fmt.Errorf("effective qdisc does not contain the fixed impairment: %s", strings.TrimSpace(state))
	}
	return state, nil
}

func fixedQdiscState(state string) bool {
	normalized := strings.ToLower(state)
	return strings.Contains(normalized, "netem") &&
		strings.Contains(normalized, "delay 40ms") &&
		strings.Contains(normalized, "rate 100mbit") &&
		strings.Contains(normalized, "limit 1000")
}

func exactContainerSet(observed []string, expected map[string]string) bool {
	if len(observed) != len(expected) {
		return false
	}
	seen := make(map[string]bool, len(observed))
	for _, containerID := range observed {
		if containerID == "" || seen[containerID] {
			return false
		}
		seen[containerID] = true
	}
	for _, containerID := range expected {
		if !seen[containerID] {
			return false
		}
	}
	return true
}

func toolingNetworkContract(networks []composeNetworkInspect, expectedName string, expectedPeers map[string]string) bool {
	if !smokeNetworkContract(networks, expectedName) || len(networks[0].Containers) != len(expectedPeers) {
		return false
	}
	for _, containerID := range expectedPeers {
		if _, found := networks[0].Containers[containerID]; !found {
			return false
		}
	}
	return true
}

func toolingPeerSetMatches(roles map[string]toolingRoleResult) bool {
	if len(roles) != len(toolingRoles) || roles["tracer-alpha"].ObservedPeer != "beta" || roles["tracer-beta"].ObservedPeer != "alpha" {
		return false
	}
	for _, role := range []string{"shape-alpha", "shape-beta", "capture-alpha"} {
		if roles[role].ObservedPeer != "" {
			return false
		}
	}
	return true
}

func validateCaptureStartup(running bool, startErr error) error {
	if startErr != nil {
		return fmt.Errorf("start packet capture: %w", startErr)
	}
	if !running {
		return errors.New("packet-capture process exited before readiness")
	}
	return nil
}

func validateCaptureEvidence(size int64, decoded, marker string) error {
	if size <= 24 {
		return errors.New("packet capture is empty or contains only its file header")
	}
	if marker == "" || !strings.Contains(decoded, marker) {
		return errors.New("packet capture does not contain the expected synthetic tracer")
	}
	return nil
}

func validateRawCapturePath(capturePath, runDirectory, repositoryRoot string) error {
	for name, path := range map[string]string{"capture": capturePath, "run directory": runDirectory, "repository": repositoryRoot} {
		if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return fmt.Errorf("%s path must be absolute and clean", name)
		}
	}
	if !pathStrictlyWithin(capturePath, runDirectory) {
		return errors.New("raw capture is outside the owned run directory")
	}
	if pathWithinOrSame(capturePath, repositoryRoot) || pathWithinOrSame(repositoryRoot, capturePath) {
		return errors.New("raw capture intersects the repository")
	}
	return nil
}

func removeRawCapture(capturePath, ownedDirectory string) error {
	if capturePath == "" || ownedDirectory == "" || !filepath.IsAbs(capturePath) || !filepath.IsAbs(ownedDirectory) || filepath.Clean(capturePath) != capturePath || !pathStrictlyWithin(capturePath, ownedDirectory) {
		return errors.New("refusing to remove raw capture outside its owned directory")
	}
	if info, err := os.Lstat(capturePath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errors.New("raw capture is not a regular owned file")
		}
		if err := os.Remove(capturePath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Lstat(capturePath); !os.IsNotExist(err) {
		return errors.New("raw capture remains after cleanup")
	}
	return nil
}

func toolingChecksPassed(checks map[string]bool) bool {
	for _, name := range requiredToolingChecks {
		if !checks[name] {
			return false
		}
	}
	return true
}

func pathStrictlyWithin(path, parent string) bool {
	return pathWithinOrSame(path, parent) && !sameFilesystemPath(path, parent)
}

func pathWithinOrSame(path, parent string) bool {
	relative, err := filepath.Rel(parent, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func sameFilesystemPath(left, right string) bool {
	left, right = filepath.Clean(left), filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
