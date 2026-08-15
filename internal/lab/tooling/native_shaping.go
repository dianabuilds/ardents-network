package tooling

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type nativeShapingResult struct {
	SchemaVersion         string                  `json:"schema_version"`
	RunID                 string                  `json:"run_id"`
	Role                  string                  `json:"role"`
	Status                string                  `json:"status"`
	EffectiveCapabilities string                  `json:"effective_capabilities"`
	ToolLockSHA256        string                  `json:"tool_lock_sha256"`
	Tools                 map[string]observedTool `json:"tools"`
	Qdisc                 map[string]string       `json:"qdisc"`
	Failure               string                  `json:"failure,omitempty"`
}

func runNativeShaper(config nativeToolConfig, evidenceDirectory string) (runErr error) {
	result := nativeShapingResult{SchemaVersion: nativeToolRoleSchema, RunID: config.RunID, Role: config.Role, Status: "failed", Qdisc: make(map[string]string)}
	defer func() {
		if runErr == nil {
			result.Status = "passed"
		} else {
			result.Failure = runErr.Error()
		}
		runErr = errors.Join(runErr, writeBoundedJSON(filepath.Join(evidenceDirectory, "result.json"), result))
	}()
	capabilities, err := effectiveCapabilities()
	result.EffectiveCapabilities = capabilities
	if err != nil || !hasOnlyEffectiveCapability(capabilities, capabilityNetAdmin) {
		return errors.Join(err, errors.New("native shaper requires exactly NET_ADMIN"))
	}
	identity, tools, err := observeTools(toolingLockPath, "tc")
	if err != nil {
		return err
	}
	result.ToolLockSHA256, result.Tools = identity.LockSHA256, tools
	interfaces, err := nativeNetworkInterfaces()
	if err != nil {
		return err
	}
	run := func(name string, arguments ...string) ([]byte, error) {
		return exec.Command(name, arguments...).CombinedOutput()
	}
	defer func() {
		for _, name := range interfaces {
			_ = deleteShaping(run, name)
		}
	}()
	for _, name := range interfaces {
		state, err := applyNativeShaping(run, name, config)
		if err != nil {
			return err
		}
		result.Qdisc[name] = state
	}
	if err := writeToolingReady(evidenceDirectory, config.RunID, config.Role); err != nil {
		return err
	}
	if err := waitForControlFile(filepath.Join("/control", "stop"), 30*time.Minute); err != nil {
		return err
	}
	for _, name := range interfaces {
		if err := deleteShaping(run, name); err != nil {
			return err
		}
	}
	interfaces = nil
	return nil
}

func nativeNetworkInterfaces() ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(interfaces))
	for _, networkInterface := range interfaces {
		if networkInterface.Name != "lo" && networkInterface.Flags&net.FlagUp != 0 {
			result = append(result, networkInterface.Name)
		}
	}
	if len(result) == 0 {
		return nil, errors.New("native role has no active non-loopback interface")
	}
	return result, nil
}

func applyNativeShaping(run externalCommand, networkInterface string, config nativeToolConfig) (string, error) {
	arguments := nativeShapingArguments(config, networkInterface)
	output, err := run("/usr/sbin/tc", arguments...)
	if err != nil {
		return "", fmt.Errorf("apply native tc netem: %w: %s", err, strings.TrimSpace(string(output)))
	}
	observed, err := run("/usr/sbin/tc", "-details", "qdisc", "show", "dev", networkInterface)
	if err != nil {
		return "", fmt.Errorf("observe native tc netem: %w: %s", err, strings.TrimSpace(string(observed)))
	}
	state := strings.ToLower(string(observed))
	if !validNativeShapingState(state, config) {
		return "", errors.New("effective native qdisc does not match the fixed impairment")
	}
	return strings.TrimSpace(string(observed)), nil
}

func nativeShapingArguments(config nativeToolConfig, networkInterface string) []string {
	arguments := []string{"qdisc", "replace", "dev", networkInterface, "root", "netem", "limit", "1000"}
	if config.Profile == "h3-s43-impaired-v1" {
		arguments = append(arguments, "delay", "150ms", "60.8ms", "distribution", "normal",
			"loss", "random", "5%", "seed", strconv.FormatUint(uint64(config.Seed), 10), "rate", "25mbit")
	} else if config.DelayMilliseconds == 40 {
		arguments = append(arguments, "delay", "40ms")
	}
	if config.Profile == "h3-s43-impaired-v1" {
		return arguments
	}
	return append(arguments, "rate", "100mbit")
}

func validNativeShapingState(state string, config nativeToolConfig) bool {
	if !strings.Contains(state, "netem") || !strings.Contains(state, "limit 1000") {
		return false
	}
	if config.Profile == "h3-s43-impaired-v1" {
		return strings.Contains(state, "delay 150ms") && strings.Contains(state, "loss 5%") &&
			strings.Contains(state, "rate 25mbit")
	}
	return strings.Contains(state, "rate 100mbit") &&
		(config.DelayMilliseconds != 40 || strings.Contains(state, "delay 40ms"))
}
