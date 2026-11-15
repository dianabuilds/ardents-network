//go:build linux

package main

import (
	"encoding/hex"
	"github.com/dianabuilds/ardents-network/internal/successor/endpoint"
)

// The offline producer writes Installation's bound schemas. Runtime decoding
// and all private-input/state/participant effects belong to endpoint.
type installedHeadless = endpoint.HeadlessPlan
type installedSource = endpoint.SourcePlan

func installedNetworkAuthority(plan installedHeadless) (*networkAuthorityPlan, error) {
	config, err := plan.Network()
	if err != nil {
		return nil, err
	}
	authority := &networkAuthorityPlan{Root: config.Root, NetworkID: config.NetworkID, Threshold: config.Threshold, ClockObservationFile: config.ClockObservationFile}
	copy(authority.ProfileAuthority[:], config.ClosedProfileAuthority)
	// Preserve declared public pin order in the offline command's exact output.
	for _, encoded := range plan.NetworkAuthorities {
		for _, key := range config.Authorities {
			if hex.EncodeToString(key) == encoded {
				authority.Authorities = append(authority.Authorities, [32]byte(key))
				break
			}
		}
	}
	return authority, nil
}
