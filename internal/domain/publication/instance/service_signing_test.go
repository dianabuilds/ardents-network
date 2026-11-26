package instance

import (
	"bytes"
	"context"
	"crypto"
	"testing"
)

func TestServiceCertificateSigningPurposeIsClosed(t *testing.T) {
	t.Parallel()
	prefix := append(bytes.Repeat([]byte{32}, 64), "TLS 1.3, server CertificateVerify\x00"...)
	for _, size := range []int{32, 48} {
		if !tlsServerCertificateVerify(append(append([]byte(nil), prefix...), make([]byte, size)...), crypto.Hash(0)) {
			t.Fatal("TLS1.3 server purpose refused")
		}
	}
	for _, input := range [][]byte{nil, []byte("arbitrary signing"), append(prefix, make([]byte, 31)...), append(prefix, make([]byte, 49)...), append(append(bytes.Repeat([]byte{32}, 64), "TLS 1.3, client CertificateVerify\x00"...), make([]byte, 32)...)} {
		if tlsServerCertificateVerify(input, crypto.Hash(0)) {
			t.Fatal("foreign signing purpose accepted")
		}
	}
	valid := append(append([]byte(nil), prefix...), make([]byte, 32)...)
	if tlsServerCertificateVerify(valid, crypto.SHA256) || tlsServerCertificateVerify(valid, nil) {
		t.Fatal("prehashed or missing Ed25519 options accepted")
	}
}

func TestMissingServiceAttachmentCannotSupplyKeyUse(t *testing.T) {
	t.Parallel()
	var service *ServiceAttachment
	if _, err := service.ProveInitial(context.Background(), nil); err == nil {
		t.Fatal("absent Service supplied proof")
	}
	if _, err := service.Evidence(); err == nil {
		t.Fatal("absent Service supplied TLS evidence")
	}
	if service.Close() != nil {
		t.Fatal("absent Service cleanup failed")
	}
	if _, _, err := (*PublicationLifetime)(nil).BeginService(nil, nil, nil); err == nil {
		t.Fatal("absent original Publication/opening/JOIN supplied Service")
	}
}
