package main

import (
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

func loadSourceCredentials(config *state.Config, plan sourcePlan) error {
	var err error
	if config.Source.ClientCertificate, err = planfile.KeyPair(plan.ClientCertificate, plan.ClientKey); err != nil {
		return err
	}
	for index, source := range plan.Sources {
		config.Source.Addresses[index], config.Source.ServerNames[index] = source.Address, source.ServerName
		config.Source.Families[index], config.Source.EndpointHandles[index] = source.Family, source.EndpointHandle
		if err := planfile.FixedHex(source.Identity, config.Source.Identities[index][:]); err != nil {
			return err
		}
		if err := planfile.FixedHex(source.LeafKeyDigest, config.Source.LeafKeyDigests[index][:]); err != nil {
			return err
		}
		config.Source.RootPEM[index], err = planfile.Read(source.RootCA, 64<<10)
		if err != nil {
			return err
		}
	}
	return nil
}
