//go:build linux

package main

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/tests/epochfixture/assignment"
)

// These are the current authenticated Role Domain names, in canonical order.
func qualificationRoleDomains() []string {
	return []string{"initiator", "introduction", "rendezvous", "responder"}
}

func qualificationRoleFamily(network, seed [32]byte, role uint8, base string) (string, error) {
	names := map[uint8]string{1: "initiator", 2: "rendezvous", 3: "responder", 4: "introduction"}
	target, ok := names[role]
	if !ok {
		return "", errors.New("qualification fixture has an unknown Role Domain")
	}
	// Mirror the current canonical Node process fixture: select a family whose
	// actual Epoch assignment reaches the required role, never fabricate proof.
	for attempt := 0; attempt < 4096; attempt++ {
		family := base
		if attempt > 0 {
			family = fmt.Sprintf("%s-%d", base, attempt)
		}
		domain, err := assignment.Select(network, 1, seed, family, qualificationRoleDomains())
		if err != nil {
			return "", err
		}
		if domain == target {
			return family, nil
		}
	}
	return "", errors.New("qualification role family search exceeded bound")
}
