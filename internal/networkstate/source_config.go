package networkstate

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"time"
)

type sourceConfig struct {
	address    string
	serverName string
	identity   [32]byte
	family     string
	handle     string
	roots      *x509.CertPool
	leafDigest [32]byte
	client     tls.Certificate
}

type sourceServerConfig struct {
	address       string
	certificate   tls.Certificate
	clientRoots   *x509.CertPool
	clientDigests map[[32]byte]bool
	headerTimeout time.Duration
}

func configureDistribution(resolved *config, input Config) error {
	hasClient := input.SourceAddresses[0] != "" || input.SourceAddresses[1] != ""
	if hasClient {
		if err := configureSources(resolved, input); err != nil {
			return err
		}
	}
	if input.ServeAddress != "" {
		server, err := configureSourceServer(input)
		if err != nil {
			return err
		}
		resolved.server = server
	}
	return nil
}

func configureSources(resolved *config, input Config) error {
	if input.SourceAddresses[0] == "" || input.SourceAddresses[1] == "" {
		return errors.New("finite source plan requires exactly two sources")
	}
	if err := validateTLSCertificate(input.SourceClientCertificate); err != nil {
		return errors.New("source client certificate is required")
	}
	clientDigest, err := certificateTransportDigest(input.SourceClientCertificate)
	if err != nil || transportDigestIsAuthority(clientDigest, resolved.authorities) {
		return errors.New("source client transport key must be separate from Epoch signer keys")
	}
	for index := range resolved.sources {
		if len(input.SourceRootPEM[index]) == 0 || len(input.SourceRootPEM[index]) > 64<<10 {
			return errors.New("source root PEM exceeds its bound")
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(input.SourceRootPEM[index]) {
			return errors.New("source root PEM is invalid")
		}
		if err := validateLiteralAddress(input.SourceAddresses[index]); err != nil {
			return err
		}
		if !validSourceText(input.SourceServerNames[index], 253) || isZero32(input.SourceIdentities[index]) ||
			!validSourceText(input.SourceFamilies[index], 64) || !validSourceText(input.SourceEndpointHandles[index], 96) ||
			isZero32(input.SourceLeafKeyDigests[index]) {
			return errors.New("source trust-map entry is incomplete")
		}
		if input.SourceLeafKeyDigests[index] == clientDigest || transportDigestIsAuthority(input.SourceLeafKeyDigests[index], resolved.authorities) {
			return errors.New("client, source, and Epoch signer keys must be separate")
		}
		resolved.sources[index] = sourceConfig{
			address: input.SourceAddresses[index], serverName: input.SourceServerNames[index],
			identity: input.SourceIdentities[index], family: input.SourceFamilies[index],
			handle: input.SourceEndpointHandles[index], roots: roots,
			leafDigest: input.SourceLeafKeyDigests[index], client: input.SourceClientCertificate,
		}
	}
	first, second := resolved.sources[0], resolved.sources[1]
	if first.identity == second.identity || first.family == second.family || first.handle == second.handle ||
		first.address == second.address || first.leafDigest == second.leafDigest {
		return errors.New("source identities, families, handles, addresses, and keys must be distinct")
	}
	return nil
}

func configureSourceServer(input Config) (sourceServerConfig, error) {
	if err := validateLiteralAddress(input.ServeAddress); err != nil {
		return sourceServerConfig{}, err
	}
	if err := validateTLSCertificate(input.ServeCertificate); err != nil {
		return sourceServerConfig{}, errors.New("source server certificate is required")
	}
	serverDigest, err := certificateTransportDigest(input.ServeCertificate)
	if err != nil || transportDigestIsAuthority(serverDigest, input.Authorities) {
		return sourceServerConfig{}, errors.New("source server transport key must be separate from Epoch signer keys")
	}
	if len(input.ServeClientRootPEM) == 0 || len(input.ServeClientRootPEM) > 64<<10 {
		return sourceServerConfig{}, errors.New("source client root PEM exceeds its bound")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(input.ServeClientRootPEM) {
		return sourceServerConfig{}, errors.New("source client root PEM is invalid")
	}
	if len(input.ServeClientKeyDigests) == 0 || len(input.ServeClientKeyDigests) > 3 {
		return sourceServerConfig{}, errors.New("source server requires one to three client key pins")
	}
	pins := make(map[[32]byte]bool, len(input.ServeClientKeyDigests))
	for _, pin := range input.ServeClientKeyDigests {
		if isZero32(pin) || pins[pin] || pin == serverDigest || transportDigestIsAuthority(pin, input.Authorities) {
			return sourceServerConfig{}, errors.New("source client key pin is invalid or duplicated")
		}
		pins[pin] = true
	}
	timeout := input.ServeReadHeaderTimeout
	if timeout <= 0 || timeout > 3*time.Second {
		timeout = 3 * time.Second
	}
	return sourceServerConfig{
		address: input.ServeAddress, certificate: input.ServeCertificate,
		clientRoots: roots, clientDigests: pins, headerTimeout: timeout,
	}, nil
}

func certificateTransportDigest(certificate tls.Certificate) ([32]byte, error) {
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return [32]byte{}, err
	}
	return transportKeyDigest(leaf.PublicKey)
}

func transportDigestIsAuthority(digest [32]byte, authorities map[[32]byte]ed25519.PublicKey) bool {
	for _, public := range authorities {
		candidate, err := transportKeyDigest(public)
		if err == nil && candidate == digest {
			return true
		}
	}
	return false
}

func validateLiteralAddress(address string) error {
	if len(address) == 0 || len(address) > 128 {
		return errors.New("source address exceeds its bound")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) == nil || port == "" {
		return errors.New("source address must be a literal IP and port")
	}
	return nil
}

func validateTLSCertificate(certificate tls.Certificate) error {
	if len(certificate.Certificate) == 0 || len(certificate.Certificate) > 4 || certificate.PrivateKey == nil {
		return errors.New("TLS certificate chain is incomplete")
	}
	for _, raw := range certificate.Certificate {
		if len(raw) == 0 || len(raw) > 64<<10 {
			return errors.New("TLS certificate exceeds its bound")
		}
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return errors.New("TLS leaf certificate is invalid")
	}
	public, ok := leaf.PublicKey.(ed25519.PublicKey)
	private, privateOK := certificate.PrivateKey.(ed25519.PrivateKey)
	if !ok || !privateOK || len(private) != ed25519.PrivateKeySize || string(private.Public().(ed25519.PublicKey)) != string(public) {
		return errors.New("TLS certificate key is not matching Ed25519")
	}
	return nil
}

func validSourceText(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum {
		return false
	}
	for _, current := range []byte(value) {
		if current < 0x21 || current > 0x7e {
			return false
		}
	}
	return true
}

func transportKeyDigest(public any) ([32]byte, error) {
	key, ok := public.(ed25519.PublicKey)
	if !ok || len(key) != ed25519.PublicKeySize {
		return [32]byte{}, errors.New("source transport key is not Ed25519")
	}
	prefix := []byte("ardents-h3-source-transport-key-v1\x00")
	return sha256.Sum256(append(prefix, key...)), nil
}

func networkIdentityDigest(network [32]byte) [32]byte {
	prefix := []byte("ardents-h3-network-id-v1\x00")
	return sha256.Sum256(append(prefix, network[:]...))
}

func sourceExposureDigest(source sourceConfig) [32]byte {
	raw := append([]byte("ardents-h3-direct-source-exposure-v1\x00"), source.identity[:]...)
	raw = append(raw, byte(len(source.family)))
	raw = append(raw, source.family...)
	raw = append(raw, byte(len(source.handle)))
	raw = append(raw, source.handle...)
	return sha256.Sum256(raw)
}

func isZero32(value [32]byte) bool { return value == [32]byte{} }
