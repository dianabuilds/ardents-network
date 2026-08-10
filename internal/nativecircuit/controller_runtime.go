package nativecircuit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type nativeInspection struct{ FixedTopology, BoundedCapabilities bool }

type nativeContainerInspect struct {
	ID     string `json:"Id"`
	Config struct {
		User   string            `json:"User"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	HostConfig struct {
		NetworkMode    string   `json:"NetworkMode"`
		ReadonlyRootfs bool     `json:"ReadonlyRootfs"`
		Privileged     bool     `json:"Privileged"`
		CapAdd         []string `json:"CapAdd"`
		CapDrop        []string `json:"CapDrop"`
		SecurityOpt    []string `json:"SecurityOpt"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Networks map[string]json.RawMessage `json:"Networks"`
		Ports    map[string]json.RawMessage `json:"Ports"`
	} `json:"NetworkSettings"`
	State struct {
		Running  bool `json:"Running"`
		ExitCode int  `json:"ExitCode"`
	} `json:"State"`
}

func nativeCompose(ctx context.Context, layout nativeRunLayout, project string, environment []string, arguments ...string) ([]byte, error) {
	base := []string{"compose", "--project-name", project, "--file", filepath.Join(layout.repositoryRoot, "carrier-lab", "compose.yaml"), "--profile", "native"}
	command := exec.CommandContext(ctx, "docker", append(base, arguments...)...)
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("native Compose: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func nativeEnvironment(fixture nativeFixture, applicationImage, toolImage string) []string {
	return append(os.Environ(), "CARRIER_LAB_RUN="+filepath.Dir(fixture.root), "CARRIER_LAB_APPLICATION_IMAGE="+applicationImage, "CARRIER_LAB_TOOL_IMAGE="+toolImage, "CARRIER_LAB_IMAGE="+applicationImage, "TOOLING_RUN_ID=native")
}

func waitNativeReady(ctx context.Context, _ nativeFixture, paths []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ready := true
		for _, path := range paths {
			if info, err := os.Stat(path); err != nil || info.Size() == 0 || info.Size() > 32*1024*1024 {
				ready = false
				break
			}
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	return errors.New("native roles did not become ready before the deadline")
}

func waitNativeServices(ctx context.Context, layout nativeRunLayout, project string, environment, services []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var running []string
	for time.Now().Before(deadline) {
		states, err := inspectNativeServiceStates(ctx, layout, project, environment)
		if err != nil {
			return err
		}
		running = running[:0]
		for _, service := range services {
			state, found := states[service]
			if !found {
				return fmt.Errorf("native service %s is absent", service)
			}
			if state.State.Running {
				running = append(running, service)
			} else if state.State.ExitCode != 0 {
				return fmt.Errorf("native service %s exited with status %d", service, state.State.ExitCode)
			}
		}
		if len(running) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return fmt.Errorf("native services did not complete before the deadline: %s", strings.Join(running, ", "))
}

func inspectNativeServiceStates(ctx context.Context, layout nativeRunLayout, project string, environment []string) (map[string]nativeContainerInspect, error) {
	idBytes, err := nativeCompose(ctx, layout, project, environment, "ps", "--all", "--quiet")
	if err != nil {
		return nil, err
	}
	ids := strings.Fields(string(idBytes))
	if len(ids) == 0 {
		return nil, errors.New("native Compose project has no containers")
	}
	data, err := exec.CommandContext(ctx, "docker", append([]string{"inspect"}, ids...)...).Output()
	if err != nil {
		return nil, err
	}
	var inspected []nativeContainerInspect
	if err := json.Unmarshal(data, &inspected); err != nil || len(inspected) != len(ids) {
		return nil, errors.New("cannot decode native container states")
	}
	states := make(map[string]nativeContainerInspect, len(inspected))
	for _, container := range inspected {
		states[container.Config.Labels["com.docker.compose.service"]] = container
	}
	return states, nil
}

func inspectNativeProject(ctx context.Context, layout nativeRunLayout, project string, environment []string) (nativeInspection, error) {
	result := nativeInspection{FixedTopology: true, BoundedCapabilities: true}
	states, err := inspectNativeServiceStates(ctx, layout, project, environment)
	if err != nil {
		return nativeInspection{}, err
	}
	services := nativeComposeServices()
	result.FixedTopology = len(states) == len(services)
	applicationIDs := make(map[string]string, len(nativeApplicationRoles))
	for _, service := range services {
		container, found := states[service]
		if !found {
			return nativeInspection{}, fmt.Errorf("native service %s is absent from inspection", service)
		}
		result.FixedTopology = result.FixedTopology && len(container.NetworkSettings.Ports) == 0 && !strings.Contains(container.HostConfig.NetworkMode, "host")
		result.BoundedCapabilities = result.BoundedCapabilities && container.HostConfig.ReadonlyRootfs && !container.HostConfig.Privileged && containsSecurityOption(container.HostConfig.SecurityOpt, "no-new-privileges")
		if strings.HasPrefix(service, "shape-") {
			result.BoundedCapabilities = result.BoundedCapabilities && exactCapabilities(container, "NET_ADMIN")
		}
		if strings.HasPrefix(service, "capture-") {
			result.BoundedCapabilities = result.BoundedCapabilities && exactCapabilities(container, "NET_RAW")
		}
		if !strings.HasPrefix(service, "shape-") && !strings.HasPrefix(service, "capture-") {
			result.BoundedCapabilities = result.BoundedCapabilities && len(container.HostConfig.CapAdd) == 0 && len(container.HostConfig.CapDrop) == 1 && container.HostConfig.CapDrop[0] == "ALL" && len(container.NetworkSettings.Networks) >= 1
			result.FixedTopology = result.FixedTopology && exactNativeRoleNetworks(project, service, container.NetworkSettings.Networks)
			applicationIDs[service] = container.ID
		}
	}
	result.FixedTopology = result.FixedTopology && exactNativeSidecarNamespaces(states, applicationIDs)
	networksMatch, err := inspectNativeNetworks(ctx, project, applicationIDs)
	if err != nil {
		return nativeInspection{}, err
	}
	result.FixedTopology = result.FixedTopology && networksMatch
	return result, nil
}

func validNativeImageID(value string) bool {
	algorithm, digest, found := strings.Cut(value, ":")
	decoded, err := hex.DecodeString(digest)
	return found && algorithm == "sha256" && err == nil && len(decoded) == sha256.Size
}
func nativeProjectName(runID string) string {
	digest := sha256.Sum256([]byte(runID))
	return "ardents-native-" + hex.EncodeToString(digest[:8])
}
func exactCapabilities(container nativeContainerInspect, expected string) bool {
	return container.Config.User == "0:0" && len(container.HostConfig.CapAdd) == 1 && normalizedCapability(container.HostConfig.CapAdd[0]) == normalizedCapability(expected) && len(container.HostConfig.CapDrop) == 1 && normalizedCapability(container.HostConfig.CapDrop[0]) == "ALL"
}
func normalizedCapability(value string) string {
	return strings.TrimPrefix(strings.ToUpper(value), "CAP_")
}
func containsSecurityOption(values []string, expected string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, expected) {
			return true
		}
	}
	return false
}
func nativeRunnerClassification() string {
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		data, _ := os.ReadFile("/etc/os-release")
		kernel, _ := os.ReadFile("/proc/sys/kernel/osrelease")
		if strings.Contains(string(data), `VERSION_ID="26.04"`) && !strings.Contains(strings.ToLower(string(kernel)), "microsoft") {
			return "official"
		}
	}
	return "development"
}
