package route

import (
	"crypto/sha256"
	"errors"
	"net"
	"strconv"
)

func verifyCandidate(input Case, values []observation) error {
	byRole := make(map[string]observation, len(values))
	for _, value := range values {
		byRole[value.Role] = value
		if value.Error != "" || !value.PeerAuthenticated {
			return errors.New("candidate process violated its authenticated Route duty")
		}
	}
	client, publisher := byRole["client"], byRole["publisher"]
	selected, err := independentlySelect(input)
	if err != nil {
		return err
	}
	if client.Generation != input.Generation || client.Epoch != input.Epoch || client.Profile != input.Profile ||
		client.ViewRoot != input.ViewRoot || client.SelectionSeed != input.SelectionSeed || client.SelectionAt != input.SelectionAt ||
		!equalIDs(client.ExcludedIdentities, input.ExcludedIdentities) ||
		!equalStrings(client.ExcludedFamilies, input.ExcludedFamilies) ||
		!equalStrings(client.ExcludedDomains, input.ExcludedDomains) || len(client.Positions) != 4 ||
		client.CanaryLength != 32 || len(client.Canary) != 32 || sha256.Sum256(client.Canary) != client.CanaryDigest ||
		publisher.NodeID != input.PublisherID || publisher.CanaryLength != 32 || publisher.CanaryDigest != client.CanaryDigest ||
		len(publisher.Positions) != 0 || len(publisher.Canary) != 0 {
		return errors.New("client plan or publisher canary evidence violates the frozen contract")
	}
	identities, keys, families := map[[32]byte]bool{}, map[[32]byte]bool{}, map[string]bool{}
	for index, expectedRole := range roles {
		position, node := client.Positions[index], byRole[expectedRole]
		if position.Role != expectedRole || position.Domain != expectedRole || position.NodeID != selected[index].NodeID ||
			input.NodeIDs[index] != selected[index].NodeID || input.PublicKeys[index] != selected[index].PublicKey ||
			input.Families[index] != selected[index].Family || input.Endpoints[index] != selected[index].Endpoint ||
			position.PublicKey != input.PublicKeys[index] || position.Family != input.Families[index] || position.Capacity == 0 ||
			position.Endpoint != input.Endpoints[index] || !literalEndpoint(position.Endpoint) ||
			identities[position.NodeID] || keys[position.PublicKey] || families[position.Family] || node.NodeID != position.NodeID ||
			node.OpaqueBytes == 0 || node.OpaqueDigest == [32]byte{} || node.CanaryDigest != [32]byte{} ||
			len(node.Canary) != 0 || len(node.Positions) != 0 {
			return errors.New("route selection or role-local evidence violates identity/family/domain separation")
		}
		previous := input.ClientPin
		if index > 0 {
			previous = input.PublicKeys[index-1]
		}
		next := input.PublisherID
		if index < 3 {
			next = input.NodeIDs[index+1]
		}
		if node.PreviousPin != previous || node.NextNodeID != next {
			return errors.New("node received more or different Route adjacency than its role permits")
		}
		identities[position.NodeID], keys[position.PublicKey], families[position.Family] = true, true, true
	}
	if publisher.PreviousPin != input.PublicKeys[3] {
		return errors.New("publisher attachment is not bound to the selected Responder")
	}
	return nil
}

func equalIDs(left, right [][32]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func literalEndpoint(value string) bool {
	host, port, err := net.SplitHostPort(value)
	number, portErr := strconv.Atoi(port)
	return err == nil && net.ParseIP(host) != nil && portErr == nil && number >= 1 && number <= 65535
}
