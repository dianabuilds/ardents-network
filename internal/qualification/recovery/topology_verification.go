package recovery

import (
	"errors"
	"strings"
)

func verifyTopology(raw []byte) error {
	text := string(raw)
	for _, forbidden := range []string{"ports:", "network_mode: host", "extra_hosts:", "dns:",
		"/var/run/docker.sock", "host.docker.internal", "HTTP_PROXY", "HTTPS_PROXY", "privileged: true", "pid: host", "ipc: host"} {
		if strings.Contains(text, forbidden) {
			return errors.New("topology contains a forbidden ambient, proxy, DNS, host, or Docker path")
		}
	}
	blocks := composeServiceBlocks(text)
	required := []string{"client-app", "publisher-app", "client-endpoint", "publisher-endpoint", "client", "publisher",
		"initiator", "introduction", "rendezvous", "responder", "recovery-verifier"}
	for _, name := range required {
		if blocks[name] == "" {
			return errors.New("topology is missing a mandatory isolated process")
		}
	}
	for _, name := range []string{"client-app", "publisher-app", "client-endpoint", "publisher-endpoint", "recovery-verifier"} {
		block := blocks[name]
		if !strings.Contains(block, "network_mode: none") || strings.Contains(block, "networks:") {
			return errors.New("application, Endpoint, or verifier gained a network data path")
		}
	}
	if strings.Contains(blocks["client-app"], "client_route") || strings.Contains(blocks["publisher-app"], "publisher_route") ||
		strings.Contains(blocks["client-endpoint"], "publisher_app") || strings.Contains(blocks["publisher-endpoint"], "client_app") {
		return errors.New("topology contains a short or cross-Application shared-volume path")
	}
	for _, name := range []string{"client", "publisher", "initiator", "introduction", "rendezvous", "responder"} {
		if !strings.Contains(blocks[name], "route_net") {
			return errors.New("route process is outside the single internal Route network")
		}
	}
	for name, block := range blocks {
		if strings.Contains(block, "/run/ardents/evidence.json") && name != "verifier" && name != "recovery-verifier" {
			return errors.New("candidate process gained the verifier evidence path")
		}
	}
	if !strings.Contains(text, "route_net:") || !strings.Contains(text, "internal: true") ||
		strings.Count(blocks["recovery-verifier"], "/run/ardents/evidence.json") != 2 {
		return errors.New("internal network or verifier-only evidence path is incomplete")
	}
	return nil
}

func composeServiceBlocks(text string) map[string]string {
	result := map[string]string{}
	lines := strings.Split(text, "\n")
	inServices, current := false, ""
	for _, line := range lines {
		if line == "services:" {
			inServices = true
			continue
		}
		if inServices && len(line) > 0 && line[0] != ' ' {
			break
		}
		if !inServices {
			continue
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			current = strings.TrimSuffix(strings.TrimSpace(line), ":")
		}
		if current != "" {
			result[current] += line + "\n"
		}
	}
	return result
}
