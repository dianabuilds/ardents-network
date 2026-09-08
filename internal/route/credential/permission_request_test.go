package credential

import (
	"crypto/ed25519"
	"testing"
	"time"
)

func TestPreparePermissionRequestUsesIndependentHolderProof(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	request, private, err := PreparePermissionRequest(permissionIdentifier(1), permissionIdentifier(2), permissionIdentifier(3), 4,
		AllocationUser, now, [3]uint32{16, 0, 8})
	if err != nil || len(private) != ed25519.PrivateKeySize || request.Permission.HolderKey == [32]byte{} {
		t.Fatalf("prepare permission request = %+v, %d-byte key, %v", request, len(private), err)
	}
	raw, err := EncodePermissionRequest(request)
	if err != nil || len(raw) != permissionRequestSize {
		t.Fatalf("encode permission request = %d bytes, %v", len(raw), err)
	}
	decoded, err := DecodePermissionRequest(raw)
	if err != nil || decoded != request {
		t.Fatalf("decode permission request = %+v, %v", decoded, err)
	}
	changed := append([]byte(nil), raw...)
	changed[100] ^= 1
	if _, err := DecodePermissionRequest(changed); err == nil {
		t.Fatal("accepted a changed holder-signed request")
	}
	zeroPrivatePermissionKey(private)
}
