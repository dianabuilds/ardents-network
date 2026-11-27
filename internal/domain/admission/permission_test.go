package admission

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"
)

func TestPermissionCanonicalVerification(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	public, private, err := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{7}, 64)))
	if err != nil || len(private) != ed25519.PrivateKeySize {
		t.Fatalf("generate authority: %v", err)
	}
	permission := Permission{NetworkID: permissionIdentifier(1), IssuerNodeID: permissionIdentifier(2), DutyGeneration: 3,
		PermissionID: permissionIdentifier(4), HolderKey: permissionIdentifier(5), NotBefore: now.Truncate(time.Hour),
		NotAfter: now.Truncate(time.Hour).Add(time.Hour), Maxima: [3]uint32{32, 0, 16}}
	copy(permission.Signature[:], ed25519.Sign(private, PermissionTranscript(permission)))
	raw, err := EncodePermission(permission)
	if err != nil || len(raw) != PermissionSize {
		t.Fatalf("encode permission = %d bytes, %v", len(raw), err)
	}
	decoded, err := DecodePermission(raw)
	if err != nil || decoded != permission {
		t.Fatalf("decode permission = %+v, %v", decoded, err)
	}
	if err := VerifyPermission(decoded, public, permission.NetworkID, permission.IssuerNodeID, permission.DutyGeneration, now); err != nil {
		t.Fatalf("verify permission: %v", err)
	}
	for _, test := range []struct {
		name            string
		network, issuer [32]byte
		duty            uint64
	}{
		{"network", permissionIdentifier(9), permission.IssuerNodeID, permission.DutyGeneration},
		{"issuer", permission.NetworkID, permissionIdentifier(9), permission.DutyGeneration},
		{"duty", permission.NetworkID, permission.IssuerNodeID, permission.DutyGeneration + 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := VerifyPermission(decoded, public, test.network, test.issuer, test.duty, now); err == nil {
				t.Fatalf("signed permission accepted a different expected %s", test.name)
			}
		})
	}
	changed := append([]byte(nil), raw...)
	changed[100] ^= 1
	decoded, err = DecodePermission(changed)
	if err != nil || VerifyPermission(decoded, public, permission.NetworkID, permission.IssuerNodeID, permission.DutyGeneration, now) == nil {
		t.Fatal("accepted changed permission")
	}
	if err := VerifyPermission(permission, public, permission.NetworkID, permission.IssuerNodeID, permission.DutyGeneration, permission.NotAfter); err == nil {
		t.Fatal("accepted expired permission")
	}
}

func permissionIdentifier(value byte) [32]byte {
	var identifier [32]byte
	identifier[0] = value
	return identifier
}
