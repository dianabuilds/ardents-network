//go:build linux

package main

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"time"
)

func buildSourceCredentials(config fixtureConfig, seed []byte, identities []fixtureParticipant,
	from, until time.Time) ([]fixtureSource, error) {
	clientAuthority := derivedKey(seed, "source-client-authority")
	clientRoot := "private/source/client-root.pem"
	clientAuthorityRaw, err := writeCertificateAuthority(config.Output, clientRoot,
		"issue60-source-client-root", 100, clientAuthority, from, until)
	if err != nil {
		return nil, err
	}
	clientKey := derivedKey(seed, "source-client")
	clientKeyPath, clientCertPath := "private/source/client-key.pem", "private/source/client-cert.pem"
	if err := writePrivateKey(config.Output, clientKeyPath, clientKey); err != nil {
		return nil, err
	}
	clientDigest, err := writeSignedCertificate(config.Output, clientCertPath, "issue60-source-client.test", 101,
		clientKey, clientAuthority, clientAuthorityRaw, x509.ExtKeyUsageClientAuth, from, until)
	if err != nil {
		return nil, err
	}
	sources := make([]fixtureSource, 0, len(identities))
	for index, identity := range identities {
		serverName := fmt.Sprintf("issue60-state-source-%d.test", index)
		serverAuthority := derivedKey(seed, fmt.Sprintf("source-%d-authority", index))
		serverRoot := fmt.Sprintf("private/source/%d-root.pem", index)
		serverAuthorityRaw, err := writeCertificateAuthority(config.Output, serverRoot,
			fmt.Sprintf("issue60-source-%d-root", index), int64(110+index*3), serverAuthority, from, until)
		if err != nil {
			return nil, err
		}
		serverKey := derivedKey(seed, fmt.Sprintf("source-%d-server", index))
		serverKeyPath := fmt.Sprintf("private/source/%d-key.pem", index)
		serverCertPath := fmt.Sprintf("private/source/%d-cert.pem", index)
		if err := writePrivateKey(config.Output, serverKeyPath, serverKey); err != nil {
			return nil, err
		}
		serverDigest, err := writeSignedCertificate(config.Output, serverCertPath, serverName, int64(111+index*3),
			serverKey, serverAuthority, serverAuthorityRaw, x509.ExtKeyUsageServerAuth, from, until)
		if err != nil {
			return nil, err
		}
		hostAddress, port := config.ReaderHost, 47000
		if identity.Host == "publisher" {
			hostAddress, port = config.PublisherHost, 47001
		}
		id := sha256.Sum256([]byte(serverName))
		sources = append(sources, fixtureSource{Name: identity.Name, Host: identity.Host, ID: hex32(id), StateNodeID: identity.ID,
			Endpoint: fmt.Sprintf("%s:%d", hostAddress, port), ServerName: serverName,
			MaterializationIndex: identity.MaterializationIndex, ServerCertificate: serverCertPath,
			ServerKey: serverKeyPath, ServerRootCA: serverRoot, ClientCertificate: clientCertPath,
			ClientKey: clientKeyPath, ClientRootCA: clientRoot, ServerLeafKeyDigest: hex32(serverDigest),
			ClientKeyDigest: hex.EncodeToString(clientDigest[:])})
	}
	return sources, nil
}
