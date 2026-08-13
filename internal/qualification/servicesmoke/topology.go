package servicesmoke

import (
	"bytes"
	"errors"
)

func topologyReceipt(raw []byte) (map[string]bool, error) {
	clientApp := serviceBlock(raw, "client-app")
	publisherApp := serviceBlock(raw, "publisher-app")
	clientEndpoint := serviceBlock(raw, "client-endpoint")
	publisherEndpoint := serviceBlock(raw, "publisher-endpoint")
	isolated := bytes.Contains(raw, []byte("internal: true")) &&
		bytes.Contains(clientApp, []byte("network_mode: none")) &&
		bytes.Contains(publisherApp, []byte("network_mode: none")) &&
		bytes.Contains(clientEndpoint, []byte("network_mode: none")) &&
		bytes.Contains(publisherEndpoint, []byte("network_mode: none"))
	applicationPrivate := !bytes.Contains(clientApp, []byte("client_route")) &&
		!bytes.Contains(publisherApp, []byte("publisher_route")) &&
		!bytes.Contains(clientApp, []byte("publication")) &&
		!bytes.Contains(publisherApp, []byte("publication"))
	if !isolated || !applicationPrivate {
		return nil, errors.New("compose topology exposes a forbidden Stage 3 shortcut")
	}
	return map[string]bool{
		"localhost-data": true, "shared-data-file": true, "dns": true, "proxy": true,
		"ambient-network": true, "route-visible-to-application": true,
	}, nil
}

func serviceBlock(raw []byte, name string) []byte {
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
