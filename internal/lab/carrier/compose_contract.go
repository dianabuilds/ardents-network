package carrier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type composeContainerInspect struct {
	Config struct {
		User string `json:"User"`
	} `json:"Config"`
	HostConfig struct {
		NetworkMode    string   `json:"NetworkMode"`
		ReadonlyRootfs bool     `json:"ReadonlyRootfs"`
		Privileged     bool     `json:"Privileged"`
		CapAdd         []string `json:"CapAdd"`
		CapDrop        []string `json:"CapDrop"`
		SecurityOpt    []string `json:"SecurityOpt"`
		PidsLimit      int64    `json:"PidsLimit"`
		Memory         int64    `json:"Memory"`
		NanoCPUs       int64    `json:"NanoCpus"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Ports    map[string]json.RawMessage `json:"Ports"`
		Networks map[string]json.RawMessage `json:"Networks"`
	} `json:"NetworkSettings"`
	Mounts []struct {
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		RW          bool   `json:"RW"`
	} `json:"Mounts"`
	State composeContainerState `json:"State"`
}

type composeContainerState struct {
	Running  bool `json:"Running"`
	ExitCode int  `json:"ExitCode"`
}

type composeNetworkInspect struct {
	Name       string                     `json:"Name"`
	Internal   bool                       `json:"Internal"`
	Containers map[string]json.RawMessage `json:"Containers"`
}

func inspectSmokeContainers(ctx context.Context, layout runLayout, project string, environment []string) (map[string]bool, map[string]string, error) {
	checks := map[string]bool{"single_internal_network": true, "no_host_ports": true, "security_controls": true}
	containerIDs := make(map[string]string, 2)
	expectedNetwork := project + "_adjacency"
	for _, role := range []string{"alpha", "beta"} {
		containerID, err := composeCommand(ctx, layout, project, environment, "ps", "--all", "--quiet", role)
		if err != nil {
			return nil, nil, err
		}
		containerIDs[role] = strings.TrimSpace(string(containerID))
		if containerIDs[role] == "" {
			return nil, nil, fmt.Errorf("compose did not retain the %s smoke container", role)
		}
		data, err := exec.CommandContext(ctx, "docker", "inspect", containerIDs[role]).Output()
		if err != nil {
			return nil, nil, err
		}
		var inspected []composeContainerInspect
		if err := json.Unmarshal(data, &inspected); err != nil || len(inspected) != 1 {
			return nil, nil, errors.New("cannot inspect smoke container")
		}
		container := inspected[0]
		checks["single_internal_network"] = checks["single_internal_network"] && len(container.NetworkSettings.Networks) == 1 && container.NetworkSettings.Networks[expectedNetwork] != nil
		checks["no_host_ports"] = checks["no_host_ports"] && len(container.NetworkSettings.Ports) == 0
		checks["security_controls"] = checks["security_controls"] && container.Config.User == "65532:65532" &&
			container.HostConfig.ReadonlyRootfs && !container.HostConfig.Privileged && container.HostConfig.NetworkMode != "host" &&
			len(container.HostConfig.CapDrop) == 1 && container.HostConfig.CapDrop[0] == "ALL" &&
			containsNoNewPrivileges(container.HostConfig.SecurityOpt) && container.HostConfig.PidsLimit > 0 &&
			container.HostConfig.Memory > 0 && container.HostConfig.NanoCPUs > 0 && isolatedMounts(container.Mounts, role)
	}
	networkData, err := exec.CommandContext(ctx, "docker", "network", "inspect", expectedNetwork).Output()
	if err != nil {
		return nil, nil, fmt.Errorf("inspect smoke network: %w", err)
	}
	var networks []composeNetworkInspect
	if err := json.Unmarshal(networkData, &networks); err != nil {
		return nil, nil, fmt.Errorf("decode smoke network inspection: %w", err)
	}
	checks["single_internal_network"] = checks["single_internal_network"] && smokeNetworkContract(networks, expectedNetwork)
	return checks, containerIDs, nil
}

func smokeNetworkContract(networks []composeNetworkInspect, expectedName string) bool {
	return len(networks) == 1 && networks[0].Name == expectedName && networks[0].Internal
}

func waitForSmokeContainers(ctx context.Context, containerIDs map[string]string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		allCompleted := true
		for _, role := range []string{"alpha", "beta"} {
			data, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .State}}", containerIDs[role]).Output()
			if err != nil {
				return fmt.Errorf("inspect %s smoke container: %w", role, err)
			}
			var state composeContainerState
			if err := json.Unmarshal(data, &state); err != nil {
				return fmt.Errorf("decode %s smoke container state: %w", role, err)
			}
			completed, err := smokeContainerOutcome(state)
			if err != nil {
				return fmt.Errorf("%s smoke container: %w", role, err)
			}
			allCompleted = allCompleted && completed
		}
		if allCompleted {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return errors.New("smoke containers did not exit before the deadline")
}

func smokeContainerOutcome(state composeContainerState) (bool, error) {
	if state.Running {
		return false, nil
	}
	if state.ExitCode != 0 {
		return true, fmt.Errorf("exited with status %d", state.ExitCode)
	}
	return true, nil
}
