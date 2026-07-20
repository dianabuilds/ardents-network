package process

import (
	"strings"

	hostingapi "ardents/internal/hosting/api"
)

func (n *Node) ListHostedServices() ([]hostingapi.HostedServiceSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.ListHostedServicesLocked()
}

func (n *Node) GetHostedService(id string) (hostingapi.HostedServiceStatusSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.publicationMgr.HostedServiceSnapshotLocked(id)
}

func (n *Node) GetServicePublicationStatus(id string) (hostingapi.PublicationStatusSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.publicationMgr.ServicePublicationStatusLocked(id), nil
}

func endpointProtocol(endpoint string) string {
	if strings.HasPrefix(endpoint, "/") {
		return "multiaddr"
	}
	if prefix, _, ok := strings.Cut(endpoint, "://"); ok {
		return prefix
	}
	return "unknown"
}
