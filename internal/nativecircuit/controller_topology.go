package nativecircuit

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
)

type nativeNetworkInspect struct {
	Name       string                     `json:"Name"`
	Internal   bool                       `json:"Internal"`
	Containers map[string]json.RawMessage `json:"Containers"`
}

var nativeNetworkRoles = map[string][]string{
	"user-ue":     {"user", "user-entry"},
	"ue-ui":       {"user-entry", "user-interior"},
	"ui-rv":       {"user-interior", "rendezvous"},
	"rv-si":       {"rendezvous", "service-interior"},
	"si-dse":      {"service-interior", "data-service-entry"},
	"dse-service": {"data-service-entry", "service"},
	"ui-if":       {"user-interior", "introduction-forwarder"},
	"if-in":       {"introduction-forwarder", "introduction-node"},
	"in-ii":       {"introduction-node", "introduction-interior"},
	"ii-ie":       {"introduction-interior", "introduction-entry"},
	"ie-service":  {"introduction-entry", "service"},
}

func exactNativeRoleNetworks(project, role string, actual map[string]json.RawMessage) bool {
	wanted := make(map[string]bool)
	for network, roles := range nativeNetworkRoles {
		for _, candidate := range roles {
			if candidate == role {
				wanted[project+"_"+network] = true
			}
		}
	}
	if len(actual) != len(wanted) {
		return false
	}
	for name := range actual {
		if !wanted[name] {
			return false
		}
	}
	return true
}

func exactNativeSidecarNamespaces(states map[string]nativeContainerInspect, applicationIDs map[string]string) bool {
	for service, container := range states {
		role := strings.TrimPrefix(strings.TrimPrefix(service, "shape-"), "capture-")
		if role == service {
			continue
		}
		if applicationIDs[role] == "" || container.HostConfig.NetworkMode != "container:"+applicationIDs[role] {
			return false
		}
	}
	return true
}

func inspectNativeNetworks(ctx context.Context, project string, applicationIDs map[string]string) (bool, error) {
	ids, err := exec.CommandContext(ctx, "docker", "network", "ls", "--quiet", "--filter", "label=com.docker.compose.project="+project).Output()
	if err != nil {
		return false, err
	}
	arguments := append([]string{"network", "inspect"}, strings.Fields(string(ids))...)
	if len(arguments) == 2 {
		return false, nil
	}
	data, err := exec.CommandContext(ctx, "docker", arguments...).Output()
	if err != nil {
		return false, err
	}
	var networks []nativeNetworkInspect
	if err := json.Unmarshal(data, &networks); err != nil {
		return false, err
	}
	if len(networks) != len(nativeNetworkRoles) {
		return false, nil
	}
	for _, network := range networks {
		shortName := strings.TrimPrefix(network.Name, project+"_")
		roles, found := nativeNetworkRoles[shortName]
		if !found || !network.Internal || len(network.Containers) != len(roles) {
			return false, nil
		}
		for _, role := range roles {
			if _, found := network.Containers[applicationIDs[role]]; !found {
				return false, nil
			}
		}
	}
	return true, nil
}
