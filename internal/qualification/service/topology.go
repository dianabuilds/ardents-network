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
	isolated := bytes.Contains(raw, []byte("internal: true")) &&
		bytes.Contains(clientApp, []byte("network_mode: none")) &&
		bytes.Contains(publisherApp, []byte("network_mode: none")) &&
		bytes.Contains(clientEndpoint, []byte("network_mode: none")) &&
		bytes.Contains(publisherEndpoint, []byte("network_mode: none"))
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
			bytes.Contains(block, []byte("ipv4_address: "+address))
	}
	noAmbient := !bytes.Contains(raw, []byte("network_mode: host")) && !bytes.Contains(raw, []byte("ports:"))
	noProxy := !bytes.Contains(bytes.ToLower(raw), []byte("http_proxy")) &&
		!bytes.Contains(bytes.ToLower(raw), []byte("https_proxy"))
	forbiddenAppTokens := []string{"route_net", "client_route", "publisher_route", "publication", "administration", "introduction_ack", "lifecycle"}
	for _, token := range forbiddenAppTokens {
		applicationPrivate = applicationPrivate && !bytes.Contains(clientApp, []byte(token)) &&
			!bytes.Contains(publisherApp, []byte(token))
	}
	exactServices := topologyServices(raw)
	expected := []string{"client", "client-app", "client-endpoint", "hostile-sibling", "initiator", "introduction",
		"negative-suite", "publication-operator", "publisher", "publisher-app", "publisher-endpoint", "rendezvous",
		"responder", "verifier", "volume-init"}
	if !isolated || !applicationPrivate || !allRoles || !noAmbient || !noProxy || !equalTopologyNames(exactServices, expected) {
		return nil, errors.New("retained topology contains or cannot exclude a forbidden Stage 3 shortcut")
	}
	return map[string]bool{
		"localhost-data": isolated, "shared-data-file": applicationPrivate, "dns": noAmbient, "proxy": noProxy,
		"ambient-network": noAmbient, "route-visible-to-application": applicationPrivate,
	}, nil
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
