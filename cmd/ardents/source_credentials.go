package main

import (
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func loadSourceCredentials(config *state.Config, plan sourcePlan) error {
	var err error
	if config.Source.ClientCertificate, err = readOperatorKeyPair(plan.ClientCertificate, plan.ClientKey); err != nil {
		return err
	}
	for index, source := range plan.Sources {
		declared := &config.Source.Sources[index]
		declared.Address, declared.ServerName = source.Address, source.ServerName
		declared.Family, declared.EndpointHandle = source.Family, source.EndpointHandle
		if err := decodeOperatorFixedHex(source.Identity, declared.Identity[:]); err != nil {
			return err
		}
		if err := decodeOperatorFixedHex(source.LeafKeyDigest, declared.LeafKeyDigest[:]); err != nil {
			return err
		}
		declared.RootPEM, err = readOperatorInput(source.RootCA, 64<<10)
		if err != nil {
			return err
		}
	}
	return nil
}
