package epoch

import networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"

// Select returns the domain with the lowest canonical assignment digest.
func selectEpochDomain(network [32]byte, epoch uint64, seed [32]byte, family string, domains []string) (string, error) {
	return networkdomain.AssignRoleDomain(network, epoch, seed, family, domains)
}

// Digest returns the canonical assignment commitment for one family/domain pair.
func epochAssignmentDigest(network [32]byte, epoch uint64, seed [32]byte, family, domain string) [32]byte {
	return networkdomain.RoleAssignmentDigest(network, epoch, seed, family, domain)
}
