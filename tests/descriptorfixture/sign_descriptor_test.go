package descriptorfixture

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
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
