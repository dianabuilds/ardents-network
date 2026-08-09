package tooling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func readToolingResults(layout runLayout) (map[string]toolingRoleResult, error) {
	results := make(map[string]toolingRoleResult, len(toolingRoles))
	for _, role := range toolingRoles {
		path := filepath.Join(layout.runDir, role, "result.json")
		info, err := os.Stat(path)
		if err != nil || info.Size() > smokeEvidenceCap {
			return nil, fmt.Errorf("missing or oversized tooling result for %s", role)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var result toolingRoleResult
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		if result.SchemaVersion != toolingResultSchema || result.RunID != layout.runID || result.Role != role {
			return nil, fmt.Errorf("invalid tooling result identity for %s", role)
		}
		results[role] = result
	}
	return results, nil
}

func evaluateToolingResults(layout runLayout, roles map[string]toolingRoleResult, summary *toolingSmokeSummary) error {
	identity, err := readToolLock(carrierLabToolLockPath(layout.repositoryRoot))
	if err != nil {
		return err
	}
	summary.ToolLockSHA256 = identity.LockSHA256
	for role, result := range roles {
		if result.Status != "passed" {
			return fmt.Errorf("%s failed: %s", role, result.Failure)
		}
		summary.EffectiveCapabilities[role] = result.EffectiveCapabilities
		if len(result.Tools) > 0 && result.ToolLockSHA256 != identity.LockSHA256 {
			return fmt.Errorf("%s used a different tool lock", role)
		}
		for name, tool := range result.Tools {
			if err := identity.verifyObservation(toolObservation{Name: name, Version: tool.Version, Path: tool.Path, SHA256: tool.SHA256}); err != nil {
				return err
			}
			if summary.Tools == nil {
				summary.Tools = make(map[string]observedTool)
			}
			summary.Tools[name] = tool
		}
	}
	summary.Checks["tool_identity"] = len(summary.Tools) == 3
	for _, role := range []string{"alpha", "beta"} {
		result := roles["shape-"+role]
		summary.QdiscState[role] = result.QdiscState
		summary.Checks["shaping_"+role] = fixedQdiscState(result.QdiscState)
	}
	summary.Checks["peer_set"] = toolingPeerSetMatches(roles)
	capture := roles["capture-alpha"]
	summary.CaptureSHA256, summary.CaptureBytes = capture.CaptureSHA256, capture.CaptureBytes
	summary.Checks["capture_started"] = capture.Status == "passed"
	summary.Checks["capture_nonempty"] = capture.CaptureBytes > 24 && validSHA256String(capture.CaptureSHA256)
	summary.Checks["capture_tracer"] = capture.CaptureTracer
	summary.Checks["raw_capture_removed"] = capture.RawCaptureRemoved && toolingPathAbsent(filepath.Join(layout.runDir, "raw-capture", "alpha-link.pcap"))
	return nil
}

func inspectToolingIsolation(ctx context.Context, layout runLayout, project string, environment []string) (bool, error) {
	containerIDs := make(map[string]string, len(toolingRoles))
	containers := make(map[string]composeContainerInspect, len(toolingRoles))
	for _, role := range toolingRoles {
		output, err := toolingComposeCommand(ctx, layout, project, environment, "ps", "--all", "--quiet", role)
		if err != nil {
			return false, err
		}
		fields := strings.Fields(string(output))
		if len(fields) != 1 {
			return false, fmt.Errorf("compose retained an invalid container set for %s", role)
		}
		containerIDs[role] = fields[0]
		data, err := exec.CommandContext(ctx, "docker", "inspect", containerIDs[role]).Output()
		if err != nil {
			return false, err
		}
		var decoded []composeContainerInspect
		if err := json.Unmarshal(data, &decoded); err != nil || len(decoded) != 1 {
			return false, errors.New("cannot inspect tooling container")
		}
		containers[role] = decoded[0]
	}
	allOutput, err := toolingComposeCommand(ctx, layout, project, environment, "ps", "--all", "--quiet")
	if err != nil {
		return false, err
	}
	if !exactContainerSet(strings.Fields(string(allOutput)), containerIDs) {
		return false, errors.New("tooling compose project contains an unexpected container set")
	}
	passed := true
	for _, role := range toolingRoles {
		container := containers[role]
		passed = passed && container.HostConfig.ReadonlyRootfs && !container.HostConfig.Privileged && len(container.HostConfig.CapDrop) == 1 && container.HostConfig.CapDrop[0] == "ALL" && containsNoNewPrivileges(container.HostConfig.SecurityOpt) && len(container.NetworkSettings.Ports) == 0
		switch role {
		case "tracer-alpha", "tracer-beta":
			passed = passed && container.Config.User == "65532:65532" && len(container.HostConfig.CapAdd) == 0 && len(container.NetworkSettings.Networks) == 1
		case "shape-alpha":
			passed = passed && container.Config.User == "0:0" && exactCapability(container.HostConfig.CapAdd, "NET_ADMIN") && container.HostConfig.NetworkMode == "container:"+containerIDs["tracer-alpha"]
		case "shape-beta":
			passed = passed && container.Config.User == "0:0" && exactCapability(container.HostConfig.CapAdd, "NET_ADMIN") && container.HostConfig.NetworkMode == "container:"+containerIDs["tracer-beta"]
		case "capture-alpha":
			passed = passed && container.Config.User == "0:0" && exactCapability(container.HostConfig.CapAdd, "NET_RAW") && container.HostConfig.NetworkMode == "container:"+containerIDs["tracer-alpha"]
		}
	}
	networkName := project + "_tooling-link"
	data, err := exec.CommandContext(ctx, "docker", "network", "inspect", networkName).Output()
	if err != nil {
		return false, err
	}
	var networks []composeNetworkInspect
	if err := json.Unmarshal(data, &networks); err != nil {
		return false, err
	}
	expectedPeers := map[string]string{"tracer-alpha": containerIDs["tracer-alpha"], "tracer-beta": containerIDs["tracer-beta"]}
	return passed && toolingNetworkContract(networks, networkName, expectedPeers), nil
}
