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
	for _, role := range []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"} {
		allRoles = allRoles && len(topologyServiceBlock(raw, role)) != 0
	}
	noAmbient := !bytes.Contains(raw, []byte("network_mode: host")) && !bytes.Contains(raw, []byte("ports:"))
	noProxy := !bytes.Contains(bytes.ToLower(raw), []byte("http_proxy")) &&
		!bytes.Contains(bytes.ToLower(raw), []byte("https_proxy"))
	if !isolated || !applicationPrivate || !allRoles || !noAmbient || !noProxy {
		return nil, errors.New("retained topology contains or cannot exclude a forbidden Stage 3 shortcut")
	}
	return map[string]bool{
		"direct": allRoles, "shortened": allRoles, "localhost-data": isolated,
		"shared-data-file": applicationPrivate, "dns": noAmbient, "proxy": noProxy,
		"ambient-network": noAmbient, "route-visible-to-application": applicationPrivate,
	}, nil
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
