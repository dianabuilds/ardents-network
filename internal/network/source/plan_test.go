package source

import (
	"crypto/tls"
	"strings"
	"testing"
	"time"
)

func TestNewRejectsPartiallyDeclaredHalves(t *testing.T) {
	cases := []struct {
		name   string
		config Config
	}{
		{"source identity", Config{Sources: [2]Source{{Identity: [32]byte{1}}}}},
		{"source family", Config{Sources: [2]Source{{Family: "declared"}}}},
		{"source server name", Config{Sources: [2]Source{{ServerName: "source.test"}}}},
		{"source endpoint handle", Config{Sources: [2]Source{{EndpointHandle: "declared"}}}},
		{"source leaf pin", Config{Sources: [2]Source{{LeafKeyDigest: [32]byte{1}}}}},
		{"second source trust root", Config{Sources: [2]Source{{}, {RootPEM: []byte("declared")}}}},
		{"client certificate", Config{ClientCertificate: tls.Certificate{Certificate: [][]byte{{1}}}}},
		{"source order seed", Config{OrderSeed: [32]byte{1}}},
		{"server certificate", Config{ServeCertificate: tls.Certificate{Certificate: [][]byte{{1}}}}},
		{"server client root", Config{ServeClientRootPEM: []byte("declared")}},
		{"server client pin", Config{ServeClientKeyDigests: [][32]byte{{1}}}},
		{"server header timeout", Config{ServeHeaderTimeout: time.Second}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := New(test.config, nil); err == nil {
				t.Fatal("incomplete declared Source half was silently treated as absent")
			}
		})
	}
	if _, details, err := New(Config{MaterialIndex: 3, VerificationClock: time.Now}, nil); err != nil || details.Configured || details.Serving {
		t.Fatalf("empty Source halves = %+v, %v", details, err)
	}
}

func TestSourceAddressRequiresUsableTCPPort(t *testing.T) {
	for _, address := range []string{"127.0.0.1:service", "127.0.0.1:65536", "[::1]:-1", "127.0.0.1:+80", "127.0.0.1:80x", "127.0.0.1:0", "[::1]:00000"} {
		if _, _, err := New(Config{ServeAddress: address, VerificationClock: time.Now}, nil); err == nil || !strings.Contains(err.Error(), "source address") {
			t.Errorf("Source plan accepted invalid TCP address %q: %v", address, err)
		}
	}
	for _, address := range []string{"127.0.0.1:1", "[::1]:65535"} {
		if err := validateAddress(address); err != nil {
			t.Errorf("valid TCP address %q: %v", address, err)
		}
	}
}
