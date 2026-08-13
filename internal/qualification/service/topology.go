package service

import (
	"bytes"
	"errors"
)

func validateTopology(raw []byte) (map[string]bool, error) {
	clientApp := topologyServiceBlock(raw, "client-app")
	publisherApp := topologyServiceBlock(raw, "publisher-app")
	clientEndpoint := topologyServiceBlock(raw, "client-endpoint")
	publisherEndpoint := topologyServiceBlock(raw, "publisher-endpoint")
	publicationOperator := topologyServiceBlock(raw, "publication-operator")
	hostileSibling := topologyServiceBlock(raw, "hostile-sibling")
	isolated := bytes.Contains(raw, []byte("internal: true")) &&
		bytes.Contains(clientApp, []byte("network_mode: none")) &&
		bytes.Contains(publisherApp, []byte("network_mode: none")) &&
		bytes.Contains(clientEndpoint, []byte("network_mode: none")) &&
		bytes.Contains(publisherEndpoint, []byte("network_mode: none")) &&
		bytes.Contains(publicationOperator, []byte("network_mode: none")) &&
		bytes.Contains(hostileSibling, []byte("network_mode: none"))
	applicationPrivate := !bytes.Contains(clientApp, []byte("client_route")) &&
		!bytes.Contains(publisherApp, []byte("publisher_route")) &&
		!bytes.Contains(clientApp, []byte("publication")) &&
		!bytes.Contains(publisherApp, []byte("publication"))
	allRoles := true
	routeAddresses := map[string]string{"client": "172.31.20.10", "initiator": "172.31.20.11",
		"introduction": "172.31.20.12", "rendezvous": "172.31.20.13", "responder": "172.31.20.14",
		"publisher": "172.31.20.16"}
	for role, address := range routeAddresses {
		block := topologyServiceBlock(raw, role)
		allRoles = allRoles && bytes.Contains(block, []byte("networks:\n      route_net:")) &&
			bytes.Contains(block, []byte("ipv4_address: "+address)) &&
			equalTopologyNames(topologyNetworks(block), []string{"route_net"})
	}
	operatorMounts := topologyMountTargets(publicationOperator)
	hostileMounts := topologyMountTargets(hostileSibling)
	noAmbient := !bytes.Contains(raw, []byte("network_mode: host")) && !bytes.Contains(raw, []byte("ports:"))
	noProxy := !bytes.Contains(bytes.ToLower(raw), []byte("http_proxy")) &&
		!bytes.Contains(bytes.ToLower(raw), []byte("https_proxy"))
	forbiddenAppTokens := []string{"route_net", "client_route", "publisher_route", "publication", "administration", "introduction_ack", "lifecycle"}
	for _, token := range forbiddenAppTokens {
		applicationPrivate = applicationPrivate && !bytes.Contains(clientApp, []byte(token)) &&
			!bytes.Contains(publisherApp, []byte(token)) && !bytes.Contains(hostileSibling, []byte(token))
	}
	for _, token := range []string{"route_net", "client_route", "publisher_route", "publication", "introduction_ack", "lifecycle"} {
		applicationPrivate = applicationPrivate && !bytes.Contains(publicationOperator, []byte(token))
	}
	applicationPrivate = applicationPrivate && len(hostileMounts) == 0 &&
		equalTopologyNames(operatorMounts, []string{"/run/ardents/admin"})
	exactServices := topologyServices(raw)
	expected := []string{"client", "client-app", "client-endpoint", "hostile-sibling", "initiator", "introduction",
		"negative-suite", "publication-operator", "publisher", "publisher-app", "publisher-endpoint", "rendezvous",
		"responder", "verifier", "volume-init"}
	for label, valid := range map[string]bool{"networkless-principals": isolated, "principal-mounts": applicationPrivate,
		"route-network": allRoles, "ambient-network": noAmbient, "proxy": noProxy,
		"service-set": equalTopologyNames(exactServices, expected)} {
		if !valid {
			return nil, errors.New("retained topology cannot exclude forbidden Stage 3 shortcut: " + label)
		}
	}
	return map[string]bool{
		"localhost-data": isolated, "shared-data-file": applicationPrivate, "dns": noAmbient, "proxy": noProxy,
		"ambient-network": noAmbient, "route-visible-to-application": applicationPrivate,
	}, nil
}

func topologyIPv4(raw []byte, name string) string {
	block := topologyServiceBlock(raw, name)
	for _, line := range bytes.Split(block, []byte{'\n'}) {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("ipv4_address: ")) {
			return string(bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("ipv4_address: "))))
		}
	}
	return ""
}

func topologyNetworks(block []byte) []string {
	return topologyIndentedNames(block, "    networks:", 6)
}

func topologyMountTargets(block []byte) []string {
	lines := bytes.Split(block, []byte{'\n'})
	var targets []string
	for _, line := range lines {
		trimmed := bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(line), []byte{'-'}))
		if bytes.HasPrefix(trimmed, []byte("target: ")) {
			targets = append(targets, string(bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("target: ")))))
		}
	}
	return targets
}

func topologyIndentedNames(block []byte, marker string, indent int) []string {
	lines := bytes.Split(block, []byte{'\n'})
	active := false
	var names []string
	for _, line := range lines {
		if string(line) == marker {
			active = true
			continue
		}
		if active && len(line) <= indent {
			break
		}
		if active && len(line) > indent && bytes.Equal(line[:indent], bytes.Repeat([]byte{' '}, indent)) &&
			line[indent] != ' ' && line[len(line)-1] == ':' {
			names = append(names, string(line[indent:len(line)-1]))
		}
	}
	return names
}

func topologyServices(raw []byte) []string {
	lines := bytes.Split(raw, []byte{'\n'})
	active := false
	var names []string
	for _, line := range lines {
		if bytes.Equal(line, []byte("services:")) {
			active = true
			continue
		}
		if active && len(line) > 0 && line[0] != ' ' {
			break
		}
		if active && len(line) > 3 && line[0] == ' ' && line[1] == ' ' && line[2] != ' ' && line[len(line)-1] == ':' {
			names = append(names, string(line[2:len(line)-1]))
		}
	}
	return names
}

func equalTopologyNames(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := map[string]bool{}
	for _, name := range left {
		seen[name] = true
	}
	for _, name := range right {
		if !seen[name] {
			return false
		}
	}
	return true
}

func topologyServiceBlock(raw []byte, name string) []byte {
	lines := bytes.Split(raw, []byte{'\n'})
	marker := []byte("  " + name + ":")
	var result []byte
	active := false
	for _, line := range lines {
		if bytes.Equal(line, marker) {
			active = true
			continue
		}
		if active && len(line) > 2 && line[0] == ' ' && line[1] == ' ' && line[2] != ' ' {
			break
		}
		if active {
			result = append(result, line...)
			result = append(result, '\n')
		}
	}
	return result
}
