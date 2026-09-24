//go:build linux

package endpoint

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// This test oracle reads the persisted receipt format independently of the
// journal's production decoder. It never opens or mutates the live owner.
const (
	textTokenReceiptHeaderSize = 48
	textTokenReceiptSize       = 153
)

type textTokenReceipt struct {
	digest, profile, receiver [32]byte
	duty                      uint64
	window                    time.Time
	class                     uint8
	attempt                   [32]byte
	observed                  time.Time
}

func readTextTokenReceipts(t *testing.T, root string, network [32]byte) []textTokenReceipt {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "attempts"))
	if err != nil {
		t.Fatal(err)
	}
	return parseTextTokenReceipts(t, raw, network)
}

func parseTextTokenReceipts(t *testing.T, raw []byte, network [32]byte) []textTokenReceipt {
	t.Helper()
	if len(raw) < textTokenReceiptHeaderSize || (len(raw)-textTokenReceiptHeaderSize)%textTokenReceiptSize != 0 ||
		string(raw[:8]) != "ARDTPS01" || !bytes.Equal(raw[8:40], network[:]) {
		t.Fatal("stored token receipt header, network or length invalid")
	}
	receipts := make([]textTokenReceipt, 0, (len(raw)-textTokenReceiptHeaderSize)/textTokenReceiptSize)
	seen := make(map[[32]byte]bool)
	for offset := textTokenReceiptHeaderSize; offset < len(raw); offset += textTokenReceiptSize {
		item := raw[offset : offset+textTokenReceiptSize]
		var receipt textTokenReceipt
		copy(receipt.digest[:], item[:32])
		copy(receipt.profile[:], item[32:64])
		copy(receipt.receiver[:], item[64:96])
		receipt.duty = binary.BigEndian.Uint64(item[96:104])
		receipt.window = time.Unix(int64(binary.BigEndian.Uint64(item[104:112])), 0).UTC()
		receipt.class = item[112]
		copy(receipt.attempt[:], item[113:145])
		receipt.observed = time.Unix(int64(binary.BigEndian.Uint64(item[145:153])), 0).UTC()
		if receipt.digest == [32]byte{} || seen[receipt.digest] || receipt.profile == [32]byte{} ||
			receipt.receiver == [32]byte{} || receipt.attempt == [32]byte{} || receipt.duty == 0 ||
			receipt.class < 1 || receipt.class > 3 || receipt.window.IsZero() || receipt.observed.Before(receipt.window) {
			t.Fatal("stored token receipt invalid or duplicated")
		}
		seen[receipt.digest] = true
		receipts = append(receipts, receipt)
	}
	return receipts
}

func snapshotTextTokenReceipts(t *testing.T, root string, network [32]byte) map[[32]byte]textTokenReceipt {
	t.Helper()
	receipts := readTextTokenReceipts(t, root, network)
	snapshot := make(map[[32]byte]textTokenReceipt, len(receipts))
	for _, receipt := range receipts {
		snapshot[receipt.digest] = receipt
	}
	return snapshot
}

func textTokenAttemptFromReceipt(receipt textTokenReceipt) textTokenAttempt {
	return textTokenAttempt{digest: receipt.digest, profile: receipt.profile, receiver: receipt.receiver, duty: receipt.duty,
		window: receipt.window, class: receipt.class, attempt: receipt.attempt, observed: receipt.observed}
}
