package resolution

type resolverRoleEvidence struct {
	Operation                         string
	Name                              string
	Isolation, Network, Nonce, Target [32]byte
	Relay, Gateway, Rendezvous        [32]byte
	Deadline                          int64
	Result                            string
	Generation, Revision              uint64
}

type gatewayRoleEvidence struct {
	Operation              string
	Name                   string
	Network, Nonce, Target [32]byte
	Deadline               int64
	Result                 string
	Generation, Revision   uint64
}

type relayRoleEvidence struct {
	Origin, Gateway   string
	Request, Response [32]byte
	RequestBytes      uint64
	ResponseBytes     uint64
	KeyID             byte
	Deadline          int64
}

// RoleEvidence returns the exact endpoint-local view for bounded S6 evidence.
func (resolver *resolver) RoleEvidence() resolverRoleEvidence {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	return resolver.roleEvidence
}

// RoleEvidence returns copies of exact naming-side views after decapsulation.
func (gateway *gateway) RoleEvidence() []gatewayRoleEvidence {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	return append([]gatewayRoleEvidence(nil), gateway.roleEvidence...)
}

// RoleEvidence returns copies of the opaque endpoint-adjacent transport views.
func (relay *relay) RoleEvidence() []relayRoleEvidence {
	relay.mu.Lock()
	defer relay.mu.Unlock()
	return append([]relayRoleEvidence(nil), relay.roleEvidence...)
}
