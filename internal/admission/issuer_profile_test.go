package admission

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"
	"time"
)

func TestDecodeClosedIssuerProfileRejectsSignedEmptyInvalidInterval(t *testing.T) {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{19}, ed25519.SeedSize))
	body := make([]byte, 8+32+32+8+8+2)
	copy(body, closedIssuerProfileMagic)
	body[8], body[40] = 1, 2
	// An empty interval makes the expected key count zero; it must not make
	// an equally empty inventory valid merely because the signature verifies.
	binary.BigEndian.PutUint64(body[72:80], 1_800_000_000)
	binary.BigEndian.PutUint64(body[80:88], 1_800_000_000)
	raw := append(body, ed25519.Sign(private, closedIssuerTranscript(body))...)
	if _, err := DecodeClosedIssuerProfile(raw, private.Public().(ed25519.PublicKey)); err == nil {
		t.Fatal("accepted a signed empty inventory with an invalid interval")
	}
}

// TestDecodeClosedIssuerProfileRejectsUnsignedTampering pins that a validly
// signed inventory refuses any body mutation that is not re-signed.
func TestDecodeClosedIssuerProfileRejectsUnsignedTampering(t *testing.T) {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	now := time.Unix(1_800_000_000, 0).UTC()
	body := signedProfileBody(t, private, now, now.Add(time.Hour), 3)
	raw := append(body, ed25519.Sign(private, closedIssuerTranscript(body))...)
	raw[89] ^= 0x01
	if _, err := DecodeClosedIssuerProfile(raw, private.Public().(ed25519.PublicKey)); err == nil {
		t.Fatal("accepted a tampered closed issuer profile body")
	}
}

// TestDecodeClosedIssuerProfileRejectsWrongMagic pins the exact ARDCIP01 magic
// even when the signature over the body verifies.
func TestDecodeClosedIssuerProfileRejectsWrongMagic(t *testing.T) {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	now := time.Unix(1_800_000_000, 0).UTC()
	body := signedProfileBody(t, private, now, now.Add(time.Hour), 3)
	copy(body, "ARDCIP02")
	raw := append(body, ed25519.Sign(private, closedIssuerTranscript(body))...)
	if _, err := DecodeClosedIssuerProfile(raw, private.Public().(ed25519.PublicKey)); err == nil {
		t.Fatal("accepted a closed issuer profile with wrong magic")
	}
}

// TestDecodeClosedIssuerProfileRejectsCountIntervalMismatch pins that the
// signed key count must equal the exact canonical hourly inventory.
func TestDecodeClosedIssuerProfileRejectsCountIntervalMismatch(t *testing.T) {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{11}, ed25519.SeedSize))
	now := time.Unix(1_800_000_000, 0).UTC()
	body := signedProfileBody(t, private, now, now.Add(time.Hour), 1)
	raw := append(body, ed25519.Sign(private, closedIssuerTranscript(body))...)
	if _, err := DecodeClosedIssuerProfile(raw, private.Public().(ed25519.PublicKey)); err == nil {
		t.Fatal("accepted a signed count that mismatches the interval")
	}
}

// signedProfileBody builds an exactly framed unsigned profile body with the
// declared key count; the entries carry no valid SPKI, so only framing,
// signature, magic, interval and count decisions can accept them.
func signedProfileBody(t *testing.T, private ed25519.PrivateKey, notBefore, notAfter time.Time, count int) []byte {
	t.Helper()
	body := make([]byte, 8+32+32+8+8+2)
	copy(body, closedIssuerProfileMagic)
	body[8], body[40] = 1, 2
	binary.BigEndian.PutUint64(body[72:80], uint64(notBefore.Unix()))
	binary.BigEndian.PutUint64(body[80:88], uint64(notAfter.Unix()))
	binary.BigEndian.PutUint16(body[88:90], uint16(count))
	return body
}
