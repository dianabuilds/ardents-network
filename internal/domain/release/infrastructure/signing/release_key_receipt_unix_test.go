//go:build linux || darwin

package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
)

func TestReleaseKeyReceiptReportsOnlyOrderedPublicSlots(t *testing.T) {
	public := releaseKeyInventory{
		{file: "release-key-1.pem", public: [32]byte{6}},
		{file: "release-key-2.pem", public: [32]byte{5}},
		{file: "release-key-3.pem", public: [32]byte{4}},
		{file: "release-key-4.pem", public: [32]byte{3}},
		{file: "release-key-5.pem", public: [32]byte{2}},
	}
	var want bytes.Buffer
	want.WriteString(`{"operation":"source.prepare-release-keys","ordinary_threshold":3,"independent_custody":false,"keys":[`)
	for i, slot := range public {
		key := slot.public
		if i != 0 {
			want.WriteByte(',')
		}
		fmt.Fprintf(&want, `{"file":"release-key-%d.pem","ed25519_public_hex":"%x","public_sha256":"%x"}`, i+1, key, sha256.Sum256(key[:]))
	}
	want.WriteString("]}\n")
	var output bytes.Buffer
	if err := writeReleaseKeyReceipt(t.Context(), &output, "source.prepare-release-keys", public); err != nil || !bytes.Equal(output.Bytes(), want.Bytes()) {
		t.Fatalf("receipt = %s, error = %v", &output, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	output.Reset()
	if err := writeReleaseKeyReceipt(ctx, &output, "source.prepare-release-keys", public); !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatalf("canceled receipt = %s, error = %v", &output, err)
	}
}
