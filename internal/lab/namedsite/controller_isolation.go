package namedsite

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type referenceInspect struct {
	ID     string `json:"Id"`
	Config struct {
		Labels map[string]string `json:"Labels"`
		User   string            `json:"User"`
	} `json:"Config"`
	HostConfig struct {
		NetworkMode    string   `json:"NetworkMode"`
		ReadonlyRootfs bool     `json:"ReadonlyRootfs"`
		Privileged     bool     `json:"Privileged"`
		CapDrop        []string `json:"CapDrop"`
		SecurityOpt    []string `json:"SecurityOpt"`
		Memory         int64    `json:"Memory"`
		PidsLimit      *int64   `json:"PidsLimit"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Networks map[string]json.RawMessage `json:"Networks"`
		Ports    map[string]json.RawMessage `json:"Ports"`
	} `json:"NetworkSettings"`
	Mounts []struct {
		Destination string `json:"Destination"`
		RW          bool   `json:"RW"`
	} `json:"Mounts"`
}

func (process *referenceProcess) inspectIsolation(ctx context.Context) error {
	if len(process.containerIDs) == 0 {
		if err := process.captureContainerIDs(ctx); err != nil {
			return err
		}
	}
	data, err := exec.CommandContext(ctx, "docker", append([]string{"inspect"}, process.containerIDs...)...).Output()
	if err != nil {
		return err
	}
	var containers []referenceInspect
	if err := json.Unmarshal(data, &containers); err != nil || len(containers) != len(referenceRoles) {
		return errors.New("reference Site inspection is invalid")
	}
	seen := make(map[string]bool)
	roleIDs := make(map[string]string)
	for _, container := range containers {
		role := container.Config.Labels["com.docker.compose.service"]
		seen[role] = true
		roleIDs[role] = container.ID
		application := role == "http-client" || role == "http-application"
		expectedUser := map[string]string{
			"http-client": "65532:65532", "client-endpoint": "65533:65533", "http-application": "65534:65534",
			"administration": "65535:65535", "authority": "65536:65536", "relay": "65537:65537", "gateway": "65538:65538",
		}[role]
		expectedMemory := int64(256 * 1024 * 1024)
		if application {
			expectedMemory = 512 * 1024 * 1024
		}
		noNewPrivileges := slices.Contains(container.HostConfig.SecurityOpt, "no-new-privileges") || slices.Contains(container.HostConfig.SecurityOpt, "no-new-privileges:true")
		if container.Config.User != expectedUser || !container.HostConfig.ReadonlyRootfs || container.HostConfig.Privileged || container.HostConfig.Memory != expectedMemory || container.HostConfig.PidsLimit == nil || *container.HostConfig.PidsLimit != 32 || len(container.HostConfig.CapDrop) != 1 || strings.ToUpper(container.HostConfig.CapDrop[0]) != "ALL" || !noNewPrivileges || len(container.NetworkSettings.Ports) != 0 {
			return process.hardIsolationFailure("capability_or_resource_isolation")
		}
		if !validRoleMounts(role, container.Mounts) {
			return process.hardIsolationFailure("principal_filesystem_view")
		}
		if role == "http-client" || role == "http-application" || role == "administration" || role == "authority" {
			if container.HostConfig.NetworkMode != "none" || !onlyNoneNetwork(container.NetworkSettings.Networks) {
				return process.hardIsolationFailure("networkless_principal_network")
			}
			continue
		}
		var networks []string
		for name := range container.NetworkSettings.Networks {
			networks = append(networks, strings.TrimPrefix(name, process.project+"_"))
		}
		slices.Sort(networks)
		expected := map[string][]string{"client-endpoint": {"client-relay"}, "gateway": {"relay-gateway"}, "relay": {"client-relay", "relay-gateway"}}[role]
		if !slices.Equal(networks, expected) {
			return process.hardIsolationFailure("resolution_role_network_view")
		}
	}
	for _, role := range referenceRoles {
		if !seen[role] {
			return errors.New("reference Site role is absent")
		}
	}
	if err := runActiveApplicationProbes(ctx, process, roleIDs); err != nil {
		if isHardGateFailure(err) {
			return errors.Join(err, process.writeFailedIsolation("active_application_probe"))
		}
		return err
	}
	return writeBoundedJSON(filepath.Join(process.evidence, "isolation.json"), map[string]any{
		"schema_version": "gatec-isolation-evidence/v1", "status": "completed", "roles": referenceRoles,
		"application_network_mode_none": true, "exact_resolution_networks": true, "resource_caps": true,
		"principal_filesystem_views": true, "no_new_privileges": true, "published_ports": false,
		"active_dns_escape_rejected": true, "active_socket_escape_rejected": true, "active_listener_absent": true,
	})
}

func (process *referenceProcess) hardIsolationFailure(class string) error {
	err := hardGate(errors.New("reference Site isolation gate failed: " + class))
	if evidenceErr := process.writeFailedIsolation(class); evidenceErr != nil {
		return errors.Join(err, matrixOperational(evidenceErr))
	}
	return err
}

func (process *referenceProcess) writeFailedIsolation(class string) error {
	return writeBoundedJSON(filepath.Join(process.evidence, "isolation.json"), map[string]any{
		"schema_version": "gatec-isolation-evidence/v1", "status": "failed", "failure_class": class,
	})
}

func runActiveApplicationProbes(ctx context.Context, process *referenceProcess, roleIDs map[string]string) (result error) {
	if process == nil || process.project == "" || process.image == "" {
		return errors.New("controlled isolation observer identity is incomplete")
	}
	networkName := process.project + "-escape-probe"
	observerName := process.project + "-observer"
	observerLabel := "io.ardents.gate-c.isolation-observer=" + observerName
	if output, err := exec.CommandContext(ctx, "docker", "network", "create", "--internal", "--label", "com.docker.compose.project="+process.project, "--label", observerLabel, networkName).CombinedOutput(); err != nil {
		return errors.New("create controlled isolation observer network: " + strings.TrimSpace(string(output)))
	}
	defer func() {
		result = errors.Join(result, cleanupIsolationObserver(process.project, observerName, networkName))
	}()
	observerArguments := []string{
		"run", "--detach", "--pull", "never", "--name", observerName,
		"--label", "com.docker.compose.project=" + process.project, "--label", observerLabel,
		"--network", networkName, "--network-alias", "gatec-observer",
		"--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--memory", "256m", "--pids-limit", "32", "--user", "65539:65539",
		process.image, "role", "--role", "isolation-observer",
	}
	if output, err := exec.CommandContext(ctx, "docker", observerArguments...).CombinedOutput(); err != nil {
		return errors.New("start controlled isolation observer: " + strings.TrimSpace(string(output)))
	}
	observerAddress, err := waitIsolationObserverAddress(ctx, observerName, networkName)
	if err != nil {
		return err
	}
	for _, role := range []string{"http-client", "http-application"} {
		containerID := roleIDs[role]
		if containerID == "" {
			return errors.New("application container is absent from active isolation probes")
		}
		command := exec.CommandContext(ctx, "docker", "exec", containerID, "/usr/local/bin/named-site-lab", "probe", "--kind", "application", "--observer-name", "gatec-observer", "--observer-address", observerAddress)
		if output, err := command.CombinedOutput(); err != nil {
			message := "active Application DNS, socket, or listener probe failed: " + strings.TrimSpace(string(output))
			if strings.Contains(message, "application DNS escape succeeded") || strings.Contains(message, "application socket escape succeeded") || strings.Contains(message, "application ordinary listener is present") {
				return hardGate(errors.New(message))
			}
			return errors.New(message)
		}
	}
	controlArguments := []string{
		"run", "--rm", "--pull", "never", "--network", networkName,
		"--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--memory", "256m", "--pids-limit", "32", "--user", "65540:65540",
		process.image, "role", "--role", "isolation-control",
		"--observer-name", "gatec-observer", "--observer-address", "gatec-observer:18080",
	}
	if output, err := exec.CommandContext(ctx, "docker", controlArguments...).CombinedOutput(); err != nil {
		return errors.New("controlled isolation observer was not reachable from its control: " + strings.TrimSpace(string(output)))
	}
	if output, err := exec.CommandContext(ctx, "docker", "wait", observerName).CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "0" {
		return errors.New("controlled isolation observer did not finish cleanly: " + strings.TrimSpace(string(output)))
	}
	return nil
}

func waitIsolationObserverAddress(ctx context.Context, observerName, networkName string) (string, error) {
	for {
		data, err := exec.CommandContext(ctx, "docker", "inspect", observerName).Output()
		var containers []struct {
			NetworkSettings struct {
				Networks map[string]struct {
					IPAddress string `json:"IPAddress"`
				} `json:"Networks"`
			} `json:"NetworkSettings"`
		}
		if err == nil && json.Unmarshal(data, &containers) == nil && len(containers) == 1 {
			address := containers[0].NetworkSettings.Networks[networkName].IPAddress
			if address != "" {
				return address + ":18080", nil
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func cleanupIsolationObserver(project, observerName, networkName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var cleanupErr error
	if data, err := exec.CommandContext(ctx, "docker", "inspect", observerName).Output(); err == nil {
		var containers []struct {
			Config struct {
				Labels map[string]string `json:"Labels"`
			} `json:"Config"`
		}
		if json.Unmarshal(data, &containers) != nil || len(containers) != 1 || containers[0].Config.Labels["com.docker.compose.project"] != project {
			return errors.New("controlled isolation observer ownership is invalid")
		}
		if output, removeErr := exec.CommandContext(ctx, "docker", "rm", "--force", observerName).CombinedOutput(); removeErr != nil {
			cleanupErr = errors.Join(cleanupErr, errors.New("remove controlled isolation observer: "+strings.TrimSpace(string(output))))
		}
	}
	if data, err := exec.CommandContext(ctx, "docker", "network", "inspect", networkName).Output(); err == nil {
		var networks []struct {
			Labels map[string]string `json:"Labels"`
		}
		if json.Unmarshal(data, &networks) != nil || len(networks) != 1 || networks[0].Labels["com.docker.compose.project"] != project {
			return errors.Join(cleanupErr, errors.New("controlled isolation observer network ownership is invalid"))
		}
		if output, removeErr := exec.CommandContext(ctx, "docker", "network", "rm", networkName).CombinedOutput(); removeErr != nil {
			cleanupErr = errors.Join(cleanupErr, errors.New("remove controlled isolation observer network: "+strings.TrimSpace(string(output))))
		}
	}
	observerLabel := "io.ardents.gate-c.isolation-observer=" + observerName
	if output, err := exec.CommandContext(ctx, "docker", "ps", "--all", "--quiet", "--filter", "label="+observerLabel).Output(); err != nil || strings.TrimSpace(string(output)) != "" {
		cleanupErr = errors.Join(cleanupErr, errors.New("controlled isolation observer container remains after cleanup"))
	}
	if output, err := exec.CommandContext(ctx, "docker", "network", "ls", "--quiet", "--filter", "label="+observerLabel).Output(); err != nil || strings.TrimSpace(string(output)) != "" {
		cleanupErr = errors.Join(cleanupErr, errors.New("controlled isolation observer network remains after cleanup"))
	}
	return cleanupErr
}
