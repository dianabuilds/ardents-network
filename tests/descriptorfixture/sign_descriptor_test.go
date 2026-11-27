package descriptorfixture

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func TestFixedDescriptorPreservesSignedVectors(t *testing.T) {
	for _, vector := range []struct {
		capabilities uint32
		digest       string
	}{
		{2, "afdeb6b27d14eef03f1bf20c8344e67002dee71396ce725678bef17b60a6f2c6"},
		{3, "2d8e15731b2e3d8484489893853f343c0c4c6929590855fd377df741f195d784"},
	} {
		raw, target, _, _, _ := Fixed(vector.capabilities, 0x11)
		digest := sha256.Sum256(raw)
		if hex.EncodeToString(digest[:]) != vector.digest {
			t.Errorf("signed fixture changed for capability bits %d", vector.capabilities)
		}
		if hex.EncodeToString(target[:]) != "e158e938b09b8cd7a50e49c5cd6714aec53866392233eb637c31af10c8a5dcc0" {
			t.Errorf("fixture Target changed for capability bits %d", vector.capabilities)
		}
	}
}

func TestProcessDescriptorPreservesSignedVector(t *testing.T) {
	at := time.Unix(1900000000, 0).UTC()
	end := at.Add(time.Minute)
	raw, target := Signed(Input{
		AuthorityMarker: 0x71, InstanceMarker: 0x72,
		Network: [32]byte{0x33}, Profile: [32]byte{0x44}, Introduction: [32]byte{18},
		Slot: [32]byte{0x73}, RecipientKey: [32]byte{0x74},
		ValidFrom: at, PublicationEnd: end, RecipientEnd: end, Capabilities: 3,
	})
	// The original Node process generator supplies this vector at the fixed time.
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != "d64c63d3a2d2e574fee0c22a8f04892e0d91e559e3d03b5cef458671f01df535" ||
		hex.EncodeToString(target[:]) != "c8bac325c80ec89afbca4a9b725ce4e4d02447d930ec660bb58aa921d3e51921" {
		t.Fatal("Node process fixture changed the original signed Descriptor or Target")
	}
}
