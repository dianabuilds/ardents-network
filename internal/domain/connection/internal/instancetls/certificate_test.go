package instancetls

import (
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"testing"
)

func TestInstanceCertificateRequiresExactSingleEd25519Key(t *testing.T) {
	t.Parallel()
	expected := [32]byte{9}
	certificate := &x509.Certificate{PublicKey: ed25519.PublicKey(expected[:])}
	valid := tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{certificate}}
	if err := VerifyPeer(valid, expected); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		name  string
		state tls.ConnectionState
		key   [32]byte
	}{
		{"absent expected key", valid, [32]byte{}},
		{"foreign expected key", valid, [32]byte{8}},
		{"missing certificate", tls.ConnectionState{Version: tls.VersionTLS13}, expected},
		{"nil certificate", tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{nil}}, expected},
		{"multiple certificates", tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{certificate, certificate}}, expected},
		{"TLS 1.2", tls.ConnectionState{Version: tls.VersionTLS12, PeerCertificates: valid.PeerCertificates}, expected},
		{"resumed", tls.ConnectionState{Version: tls.VersionTLS13, DidResume: true, PeerCertificates: valid.PeerCertificates}, expected},
		{"wrong algorithm", tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{{PublicKey: &rsa.PublicKey{}}}}, expected},
		{"short key", tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{{PublicKey: ed25519.PublicKey{9}}}}, expected},
	} {
		t.Run(input.name, func(t *testing.T) {
			if err := VerifyPeer(input.state, input.key); err == nil {
				t.Fatal("foreign or unsupported Instance certificate accepted")
			}
		})
	}
}
