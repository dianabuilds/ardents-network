package main

import (
	"crypto/tls"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
)

func loadSourceCredentials(config *networkstate.Config, plan sourcePlan) error {
	certificatePEM, err := readCommandFile(plan.ClientCertificate, 64<<10)
	if err != nil {
		return err
	}
	keyPEM, err := readCommandFile(plan.ClientKey, 64<<10)
	if err != nil {
		return err
	}
	config.SourceClientCertificate, err = tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		return err
	}
	for index, source := range plan.Sources {
		config.SourceAddresses[index], config.SourceServerNames[index] = source.Address, source.ServerName
		config.SourceFamilies[index], config.SourceEndpointHandles[index] = source.Family, source.EndpointHandle
		if err := decodeFixedHex(source.Identity, config.SourceIdentities[index][:]); err != nil {
			return err
		}
		if err := decodeFixedHex(source.LeafKeyDigest, config.SourceLeafKeyDigests[index][:]); err != nil {
			return err
		}
		config.SourceRootPEM[index], err = readCommandFile(source.RootCA, 64<<10)
		if err != nil {
			return err
		}
	}
	return nil
}
