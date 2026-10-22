package source

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strconv"
	"time"
)

func configureClients(plan *Plan, input Config, authorities map[[32]byte]ed25519.PublicKey) error {
	return configureClientTrust(plan, input, authorities, false)
}

func configureClientTrust(plan *Plan, input Config, authorities map[[32]byte]ed25519.PublicKey, observation bool) error {
	if input.Sources[0].Address == "" || input.Sources[1].Address == "" {
		return errors.New("finite source plan requires exactly two sources")
	}
	if err := validateCredential(input.ClientCertificate, observation); err != nil {
		return errors.New("source client certificate is required")
	}
	clientCertificate := cloneCertificate(input.ClientCertificate)
	clientDigest, err := certificateDigest(input.ClientCertificate)
	if err != nil || digestIsAuthority(clientDigest, authorities) {
		return errors.New("source client transport key must be separate from Epoch signer keys")
	}
	for index := range plan.clients {
		declared := input.Sources[index]
		resolved, err := declaredClient(declared, clientCertificate, clientDigest, authorities, input.VerificationClock)
		if err != nil {
			return err
		}
		plan.clients[index] = resolved
		plan.details.Identities[index], plan.details.Families[index], plan.details.EndpointHandles[index] =
			declared.Identity, declared.Family, declared.EndpointHandle
		raw := append([]byte("ardents-h3-direct-source-exposure-v1\x00"), declared.Identity[:]...)
		raw = append(raw, byte(len(declared.Family)))
		raw = append(raw, declared.Family...)
		raw = append(raw, byte(len(declared.EndpointHandle)))
		raw = append(raw, declared.EndpointHandle...)
		plan.details.Exposures[index] = sha256.Sum256(raw)
	}
	if plan.details.Identities[0] == plan.details.Identities[1] || plan.details.Families[0] == plan.details.Families[1] ||
		plan.details.EndpointHandles[0] == plan.details.EndpointHandles[1] || plan.clients[0].address == plan.clients[1].address ||
		plan.clients[0].leafKeyDigest == plan.clients[1].leafKeyDigest {
		return errors.New("source identities, families, handles, addresses, and keys must be distinct")
	}
	plan.details.Configured = true
	return nil
}

func declaredClient(declared Source, certificate tls.Certificate, clientDigest [32]byte, authorities map[[32]byte]ed25519.PublicKey, clock func() time.Time) (client, error) {
	roots, err := parseRoots(declared.RootPEM)
	if err != nil {
		return client{}, err
	}
	if err := validateAddress(declared.Address); err != nil {
		return client{}, err
	}
	if !validText(declared.ServerName, 253) || isZero(declared.Identity) ||
		!validText(declared.Family, 64) || !validText(declared.EndpointHandle, 96) || isZero(declared.LeafKeyDigest) {
		return client{}, errors.New("source trust-map entry is incomplete")
	}
	if _, exists := authorities[declared.Identity]; exists {
		return client{}, errors.New("source identity collides with an Epoch authority")
	}
	if declared.LeafKeyDigest == clientDigest || digestIsAuthority(declared.LeafKeyDigest, authorities) {
		return client{}, errors.New("client, source, and Epoch signer keys must be separate")
	}
	return client{address: declared.Address, serverName: declared.ServerName, roots: roots, leafKeyDigest: declared.LeafKeyDigest,
		certificate: certificate, clock: clock}, nil
}

func configureServer(input Config, authorities map[[32]byte]ed25519.PublicKey) (server, error) {
	return configureServerTrust(input, authorities, false)
}

func configureServerTrust(input Config, authorities map[[32]byte]ed25519.PublicKey, observation bool) (server, error) {
	if err := validateAddress(input.ServeAddress); err != nil {
		return server{}, err
	}
	if err := validateCredential(input.ServeCertificate, observation); err != nil {
		return server{}, errors.New("source server certificate is required")
	}
	digest, err := certificateDigest(input.ServeCertificate)
	if err != nil || digestIsAuthority(digest, authorities) {
		return server{}, errors.New("source server transport key must be separate from Epoch signer keys")
	}
	roots, err := parseRoots(input.ServeClientRootPEM)
	if err != nil {
		return server{}, err
	}
	if len(input.ServeClientKeyDigests) == 0 || len(input.ServeClientKeyDigests) > 3 {
		return server{}, errors.New("source server requires one to three client key pins")
	}
	pins := make(map[[32]byte]bool, len(input.ServeClientKeyDigests))
	for _, pin := range input.ServeClientKeyDigests {
		if isZero(pin) || pins[pin] || pin == digest || digestIsAuthority(pin, authorities) {
			return server{}, errors.New("source client key pin is invalid or duplicated")
		}
		pins[pin] = true
	}
	timeout := input.ServeHeaderTimeout
	if timeout <= 0 || timeout > 3*time.Second {
		timeout = 3 * time.Second
	}
	return server{address: input.ServeAddress, certificate: cloneCertificate(input.ServeCertificate),
		clientRoots: roots, clientDigests: pins, headerTimeout: timeout, clock: input.VerificationClock}, nil
}

func cloneCertificate(input tls.Certificate) tls.Certificate {
	clone := tls.Certificate{
		Certificate:                  make([][]byte, len(input.Certificate)),
		SupportedSignatureAlgorithms: append([]tls.SignatureScheme(nil), input.SupportedSignatureAlgorithms...),
		OCSPStaple:                   append([]byte(nil), input.OCSPStaple...),
		SignedCertificateTimestamps:  make([][]byte, len(input.SignedCertificateTimestamps)),
	}
	if key, ok := input.PrivateKey.(ed25519.PrivateKey); ok {
		clone.PrivateKey = append(ed25519.PrivateKey(nil), key...)
	}
	for index := range input.Certificate {
		clone.Certificate[index] = append([]byte(nil), input.Certificate[index]...)
	}
	for index := range input.SignedCertificateTimestamps {
		clone.SignedCertificateTimestamps[index] = append([]byte(nil), input.SignedCertificateTimestamps[index]...)
	}
	return clone
}

func parseRoots(raw []byte) (*x509.CertPool, error) {
	if len(raw) == 0 || len(raw) > 64<<10 {
		return nil, errors.New("source root PEM exceeds its bound")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(raw) {
		return nil, errors.New("source root PEM is invalid")
	}
	return roots, nil
}

func validateAddress(address string) error {
	if len(address) == 0 || len(address) > 128 {
		return errors.New("source address exceeds its bound")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) == nil || port == "" {
		return errors.New("source address must be a literal IP and port")
	}
	for _, digit := range port {
		if digit < '0' || digit > '9' {
			return errors.New("source address must be a literal IP and port")
		}
	}
	number, err := strconv.Atoi(port)
	if err != nil || number == 0 || number > 65535 {
		return errors.New("source address must be a literal IP and port")
	}
	return nil
}

func validText(value string, maximum int) bool {
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

func validateCredential(certificate tls.Certificate, observation bool) error {
	if !observation {
		return ValidateServingCredential(certificate)
	}
	return validateCredentialChain(certificate)
}

func validateCredentialChain(certificate tls.Certificate) error {
	if len(certificate.Certificate) == 0 || len(certificate.Certificate) > 4 {
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
	if !ok || len(public) != ed25519.PublicKeySize {
		return errors.New("TLS certificate key is not Ed25519")
	}
	return nil
}

// ValidateServingCredential applies the same finite Ed25519 chain/private-pair
// contract as serving, without opening a Source, accepting State or granting
// handshake-time trust. The caller retains and erases its loaded private key.
func ValidateServingCredential(certificate tls.Certificate) error {
	if err := validateCredentialChain(certificate); err != nil {
		return err
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return errors.New("TLS leaf certificate is invalid")
	}
	public, publicOK := leaf.PublicKey.(ed25519.PublicKey)
	private, ok := certificate.PrivateKey.(ed25519.PrivateKey)
	if !publicOK || !ok || len(private) != ed25519.PrivateKeySize || string(private.Public().(ed25519.PublicKey)) != string(public) {
		return errors.New("TLS certificate key is not matching Ed25519")
	}
	return nil
}

func certificateDigest(certificate tls.Certificate) ([32]byte, error) {
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return [32]byte{}, err
	}
	return keyDigest(leaf.PublicKey)
}

func digestIsAuthority(digest [32]byte, authorities map[[32]byte]ed25519.PublicKey) bool {
	for _, public := range authorities {
		candidate, err := keyDigest(public)
		if err == nil && candidate == digest {
			return true
		}
	}
	return false
}

func isZero(value [32]byte) bool { return value == [32]byte{} }
