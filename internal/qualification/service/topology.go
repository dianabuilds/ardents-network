package service

import (
	"bytes"
	"errors"
	"strings"
)

func validateTopology(raw []byte) (map[string]bool, error) {
	clientApp := topologyServiceBlock(raw, "client-app")
	publisherApp := topologyServiceBlock(raw, "publisher-app")
	clientEndpoint := topologyServiceBlock(raw, "client-endpoint")
	publisherEndpoint := topologyServiceBlock(raw, "publisher-endpoint")
	publicationOperator := topologyServiceBlock(raw, "publication-operator")
	hostileSibling := topologyServiceBlock(raw, "hostile-sibling")
	networkNames := topologyIndentedNames(raw, "networks:", 2)
	routeNetwork := topologyIndentedBlock(raw, "networks:", "route_net", 2)
	internalRoute := equalTopologyNames(networkNames, []string{"route_net"}) &&
		topologyDirectValue(routeNetwork, "internal", 4) == "true"
	networkless := true
	for _, block := range [][]byte{clientApp, publisherApp, clientEndpoint, publisherEndpoint, publicationOperator, hostileSibling} {
		networkless = networkless && topologyDirectValue(block, "network_mode", 4) == "none"
	}
	clientMounts, clientMountsValid := topologyMounts(topologyPropertyBlock(clientApp, "    volumes:", 4))
	publisherMounts, publisherMountsValid := topologyMounts(topologyPropertyBlock(publisherApp, "    volumes:", 4))
	applicationPrivate := clientMountsValid && publisherMountsValid &&
		validApplicationMounts(clientMounts, "client_app", "/run/ardents/client-app") &&
		validApplicationMounts(publisherMounts, "publisher_app", "/run/ardents/publisher-app") &&
		!bytes.Contains(clientApp, []byte("client_route")) &&
		!bytes.Contains(publisherApp, []byte("publisher_route")) &&
		!bytes.Contains(clientApp, []byte("publication")) &&
		!bytes.Contains(publisherApp, []byte("publication"))
	allRoles := true
	routeAddresses := map[string]string{"client": "172.31.20.10", "initiator": "172.31.20.11",
		"introduction": "172.31.20.12", "rendezvous": "172.31.20.13", "responder": "172.31.20.14",
		"publisher": "172.31.20.16"}
	for role, address := range routeAddresses {
		block := topologyServiceBlock(raw, role)
		routeAttachment := topologyIndentedBlock(block, "    networks:", "route_net", 6)
		allRoles = allRoles && topologyDirectValue(routeAttachment, "ipv4_address", 8) == address &&
			equalTopologyNames(topologyNetworks(block), []string{"route_net"})
	}
	operatorVolumeBlock := topologyPropertyBlock(publicationOperator, "    volumes:", 4)
	operatorMounts, operatorMountsValid := topologyMounts(operatorVolumeBlock)
	hostileHasVolumes := topologyHasDirectProperty(hostileSibling, "volumes", 4)
	noAmbient := !bytes.Contains(raw, []byte("network_mode: host")) && !bytes.Contains(raw, []byte("ports:"))
	lowerTopology := bytes.ToLower(raw)
	noProxy := true
	for _, key := range []string{"all_proxy", "ftp_proxy", "http_proxy", "https_proxy"} {
		noProxy = noProxy && !bytes.Contains(lowerTopology, []byte(key))
	}
	forbiddenAppTokens := []string{"route_net", "client_route", "publisher_route", "publication", "administration", "introduction_ack", "lifecycle"}
	for _, token := range forbiddenAppTokens {
		applicationPrivate = applicationPrivate && !bytes.Contains(clientApp, []byte(token)) &&
			!bytes.Contains(publisherApp, []byte(token)) && !bytes.Contains(hostileSibling, []byte(token))
	}
	for _, token := range []string{"route_net", "client_route", "publisher_route", "publication", "introduction_ack", "lifecycle"} {
		applicationPrivate = applicationPrivate && !bytes.Contains(publicationOperator, []byte(token))
	}
	applicationPrivate = applicationPrivate && !hostileHasVolumes && len(operatorVolumeBlock) != 0 &&
		operatorMountsValid && len(operatorMounts) == 1 &&
		operatorMounts[0] == (topologyMount{kind: "volume", source: "administration", target: "/run/ardents/admin"})
	exactServices := topologyServices(raw)
	expected := []string{"client", "client-app", "client-endpoint", "hostile-sibling", "initiator", "introduction",
		"negative-suite", "publication-operator", "publisher", "publisher-app", "publisher-endpoint", "rendezvous",
		"responder", "verifier", "volume-init"}
	for label, valid := range map[string]bool{"internal-route-network": internalRoute, "networkless-principals": networkless,
		"principal-mounts": applicationPrivate, "route-network": allRoles, "ambient-network": noAmbient, "proxy": noProxy,
		"service-set": equalTopologyNames(exactServices, expected)} {
		if !valid {
			return nil, errors.New("retained topology cannot exclude forbidden Stage 3 shortcut: " + label)
		}
	}
	return map[string]bool{
		"localhost-data": networkless, "shared-data-file": applicationPrivate, "dns": noAmbient, "proxy": noProxy,
		"ambient-network": noAmbient, "route-visible-to-application": applicationPrivate,
	}, nil
}

func validApplicationMounts(mounts []topologyMount, socketSource, socketTarget string) bool {
	if len(mounts) != 3 {
		return false
	}
	expected := map[string]topologyMount{
		socketTarget:                          {kind: "volume", source: socketSource, target: socketTarget},
		"/run/ardents/workload/client.hex":    {kind: "bind", target: "/run/ardents/workload/client.hex", readOnly: true},
		"/run/ardents/workload/publisher.hex": {kind: "bind", target: "/run/ardents/workload/publisher.hex", readOnly: true},
	}
	seen := map[string]bool{}
	for _, mount := range mounts {
		want, ok := expected[mount.target]
		if !ok || seen[mount.target] || mount.kind != want.kind || mount.readOnly != want.readOnly {
			return false
		}
		if mount.target == socketTarget {
			if mount.source != socketSource {
				return false
			}
		} else {
			seed := strings.TrimSuffix(strings.TrimPrefix(mount.target, "/run/ardents/workload/"), ".hex")
			source := strings.ReplaceAll(mount.source, "\\", "/")
			if !strings.HasSuffix(source, "/"+seed+"-seed.hex") {
				return false
			}
		}
		seen[mount.target] = true
	}
	return len(seen) == len(expected)
}

func topologyIPv4(raw []byte, name string) string {
	block := topologyServiceBlock(raw, name)
	routeAttachment := topologyIndentedBlock(block, "    networks:", "route_net", 6)
	return topologyDirectValue(routeAttachment, "ipv4_address", 8)
}

func topologyNetworks(block []byte) []string {
	return topologyIndentedNames(block, "    networks:", 6)
}
