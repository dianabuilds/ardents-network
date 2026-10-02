package nodeidentity

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func identityFixture(t testing.TB) (Binding, []byte, []byte) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, 32))
	der, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	b := Binding{Network: [32]byte{1}, Node: [32]byte{2}, Signer: [32]byte(key.Public().(ed25519.PublicKey))}
	return b, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), der
}
func TestIdentityEncodingPinnedAndBounded(t *testing.T) {
	b, raw, der := identityFixture(t)
	got, e := decodePEM(raw, b)
	if e != nil || !bytes.Equal(got, der) {
		t.Fatal(e)
	}
	if got, e := decodePEM(bytes.ReplaceAll(raw, []byte("\n"), []byte("\r\n")), b); e != nil || !bytes.Equal(got, der) {
		t.Fatal("CRLF PEM", e)
	}
	bad := b
	bad.Signer[0] ^= 1
	if _, e = decodePEM(raw, bad); e == nil {
		t.Fatal("wrong pin")
	}
	other, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	otherDER, e := x509.MarshalPKCS8PrivateKey(other)
	if e != nil {
		t.Fatal(e)
	}
	for _, input := range [][]byte{nil, append(append([]byte(nil), raw...), raw...), append([]byte("junk"), raw...), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: otherDER}), make([]byte, (64<<10)+1)} {
		if _, e := decodePEM(input, b); e == nil {
			t.Fatal("invalid PEM accepted")
		}
	}
}
func FuzzNodeIdentityPEM(f *testing.F) {
	b, valid, _ := identityFixture(f)
	f.Add(valid)
	f.Add([]byte("-----BEGIN PRIVATE KEY-----\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 64<<10+1 {
			return
		}
		der, e := decodePEM(raw, b)
		if e == nil {
			key, e := parseKey(der, b)
			if e != nil {
				t.Fatal("invalid imported key")
			}
			clear(key)
			clear(der)
		}
	})
}
