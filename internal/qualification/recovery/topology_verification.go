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
		"initiator", "introduction", "rendezvous", "responder", "carrier-fault", "recovery-verifier"}
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
		if !strings.Contains(blocks[name], "restart: \"no\"") {
			return errors.New("selected Route process permits an unobserved restart")
		}
	}
	if !strings.Contains(blocks["rendezvous"], "172.31.21.13") || !strings.Contains(blocks["responder"], "172.31.21.14") ||
		!strings.Contains(blocks["rendezvous"], "carrier_net") || !strings.Contains(blocks["responder"], "carrier_net") {
		return errors.New("native Carrier endpoint addresses differ from the frozen topology")
	}
	for _, name := range []string{"client", "publisher", "initiator", "introduction"} {
		if strings.Contains(blocks[name], "carrier_net") {
			return errors.New("dedicated Carrier network reached a non-adjacent process")
		}
	}
	controller := blocks["carrier-fault"]
	if strings.Contains(controller, "volumes:") || strings.Contains(controller, "networks:") ||
		strings.Contains(controller, "pid:") || strings.Contains(controller, "ipc:") ||
		!strings.Contains(controller, "network_mode: service:rendezvous") ||
		!strings.Contains(controller, "ardents-qualify") || !strings.Contains(controller, "carrier-fault") ||
		!strings.Contains(controller, "read_only: true") || !strings.Contains(controller, "restart: \"no\"") ||
		!strings.Contains(controller, "user: \"0:0\"") || !strings.Contains(controller, "pids_limit: 16") ||
		!strings.Contains(controller, "mem_limit: \"33554432\"") || !strings.Contains(controller, "cpus: 0.25") ||
		!exactComposeList(controller, "cap_add", []string{"NET_ADMIN"}) ||
		!exactComposeList(controller, "cap_drop", []string{"ALL"}) ||
		!exactComposeList(controller, "security_opt", []string{"no-new-privileges:true"}) {
		return errors.New("fault controller gained a data path or lost its exact socket authority")
	}
	for name, block := range blocks {
		if strings.Contains(block, "/run/ardents/evidence.json") && name != "verifier" && name != "recovery-verifier" {
			return errors.New("candidate process gained the verifier evidence path")
		}
	}
	if !strings.Contains(text, "route_net:") || !strings.Contains(text, "carrier_net:") ||
		strings.Count(text, "internal: true") < 2 ||
		strings.Count(blocks["recovery-verifier"], "/run/ardents/evidence.json") != 2 {
		return errors.New("internal network or verifier-only evidence path is incomplete")
	}
	return nil
}

func exactComposeList(block, key string, expected []string) bool {
	lines := strings.Split(block, "\n")
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+": [") && strings.HasSuffix(trimmed, "]") {
			value := strings.TrimSuffix(strings.TrimPrefix(trimmed, key+": ["), "]")
			return len(expected) == 1 && value == expected[0]
		}
		if trimmed != key+":" {
			continue
		}
		var values []string
		for next := index + 1; next < len(lines); next++ {
			value := strings.TrimSpace(lines[next])
			if !strings.HasPrefix(value, "- ") {
				break
			}
			values = append(values, strings.TrimPrefix(value, "- "))
		}
		return len(values) == len(expected) && strings.Join(values, "\x00") == strings.Join(expected, "\x00")
	}
	return false
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
