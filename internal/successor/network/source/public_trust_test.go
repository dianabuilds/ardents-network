//go:build linux

package source

import (
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/pem"
	"testing"
	"time"
)

func TestPublicTrustObservationPurposesAndCollisions(t *testing.T) {
	now := time.Date(2032, 2, 3, 4, 5, 6, 0, time.UTC)
	for _, fault := range []string{"healthy", "external-pem", "ca", "name", "server-role", "client-role", "expired", "future", "server-pin", "client-pin", "identity", "family", "handle", "address", "key", "authority-key", "authority-identity", "mixed", "malformed-first", "private", "empty-root"} {
		t.Run(fault, func(t *testing.T) {
			f := newSourceTLSFixture(t, now)
			f.authority.nextSeed += 10
			second := f.authority.issueServer(t, now.Add(-time.Hour), now.Add(time.Hour), f.serverName)
			encode := func(c tls.Certificate) []byte {
				return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Certificate[0]})
			}
			input := PublicTrust{ClientCertificatePEM: encode(f.client), ClientRootPEM: f.authority.rootPEM, ClientKeyDigest: f.clientPin}
			for i, c := range []tls.Certificate{f.server, second} {
				input.Sources[i] = PublicSource{Source: Source{Address: "127.0.0.1:1001", ServerName: f.serverName, Identity: [32]byte{byte(i + 1)}, Family: string(rune('a' + i)), EndpointHandle: string(rune('a' + i)), RootPEM: f.authority.rootPEM, LeafKeyDigest: sourceTLSLeafKeyDigest(t, c)}, CertificatePEM: encode(c)}
			}
			input.Sources[1].Source.Address = "127.0.0.1:1002"
			authorities := map[[32]byte]ed25519.PublicKey{}
			switch fault {
			case "external-pem":
				block, _ := pem.Decode(input.ClientRootPEM)
				input.ClientRootPEM = []byte("-----BEGIN CERTIFICATE-----\r\n" + base64.StdEncoding.EncodeToString(block.Bytes) + "\r\n-----END CERTIFICATE-----\r\n")
			case "ca":
				other := newSourceTLSAuthority(t, now, 100)
				input.Sources[0].Source.RootPEM = other.rootPEM
			case "name":
				input.Sources[0].Source.ServerName = "other.test"
			case "server-role":
				input.Sources[0].CertificatePEM = encode(f.client)
			case "client-role":
				input.ClientCertificatePEM = encode(f.server)
			case "expired":
				input.Sources[0].CertificatePEM = encode(f.authority.issueServer(t, now.Add(-2*time.Hour), now.Add(-time.Hour), f.serverName))
			case "future":
				input.Sources[0].CertificatePEM = encode(f.authority.issueServer(t, now.Add(time.Hour), now.Add(2*time.Hour), f.serverName))
			case "server-pin":
				input.Sources[0].Source.LeafKeyDigest[0] ^= 1
			case "client-pin":
				input.ClientKeyDigest[0] ^= 1
			case "identity":
				input.Sources[1].Source.Identity = input.Sources[0].Source.Identity
			case "family":
				input.Sources[1].Source.Family = input.Sources[0].Source.Family
			case "handle":
				input.Sources[1].Source.EndpointHandle = input.Sources[0].Source.EndpointHandle
			case "address":
				input.Sources[1].Source.Address = input.Sources[0].Source.Address
			case "key":
				input.Sources[1].CertificatePEM = input.Sources[0].CertificatePEM
				input.Sources[1].Source.LeafKeyDigest = input.Sources[0].Source.LeafKeyDigest
			case "authority-key":
				authorities[[32]byte{90}] = f.client.PrivateKey.(ed25519.PrivateKey).Public().(ed25519.PublicKey)
			case "authority-identity":
				authorities[input.Sources[0].Source.Identity] = make(ed25519.PublicKey, 32)
			case "mixed":
				input.Sources[0].CertificatePEM = append([]byte("ignored"), input.Sources[0].CertificatePEM...)
			case "malformed-first":
				input.Sources[0].CertificatePEM = append([]byte("-----BEGIN CERTIFICATE-----\ninvalid!\n"), input.Sources[0].CertificatePEM...)
			case "private":
				input.ClientRootPEM = append(input.ClientRootPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1}})...)
			case "empty-root":
				input.ClientRootPEM = nil
			}
			err := ObservePublicTrust(input, authorities, now)
			if (err == nil) != (fault == "healthy" || fault == "external-pem") {
				t.Fatal("observation outcome", err)
			}
		})
	}
}
