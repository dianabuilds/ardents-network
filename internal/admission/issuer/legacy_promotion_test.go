package issuer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"
)

func TestLegacyIssuerPromotionRetainsIndependentBytesAfterUncertainReopen(t *testing.T) {
	root := t.TempDir()
	network, node, profile := [32]byte{1}, [32]byte{2}, [32]byte{3}
	legacy, promoted, window := legacyPromotionBytes()
	path := filepath.Join(root, "closed-token-ledger")
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("promotion reopen durability unavailable")
	called := false
	ledger, err := openClosedTokenIssuerLedgerWithBarrier(root, network, node, profile, func(path string) error {
		called = true
		if path != root {
			t.Fatal("barrier used another root")
		}
		return failure
	})
	if !called || ledger != nil || !errors.Is(err, failure) {
		t.Fatalf("uncertain promotion exposed ledger: called=%t ledger=%v err=%v", called, ledger, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, promoted) {
		t.Fatalf("promotion changed the independent retained-byte oracle: %v", err)
	}
	checkPromotedLegacyQuota(t, root, window)
}

func legacyPromotionBytes() (legacy, promoted []byte, window time.Time) {
	network, node, profile := [32]byte{1}, [32]byte{2}, [32]byte{3}
	window = time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	// Independent on-disk oracle: neither the fixture nor expected promotion
	// uses the production header or reservation encoder. v1 has no kind byte;
	// every retained v1 debit must acquire the literal bootstrap kind (1).
	legacy, promoted = []byte("ARDILG01"), []byte("ARDILG02")
	for _, identity := range [][32]byte{network, node, profile} {
		legacy = append(legacy, identity[:]...)
		promoted = append(promoted, identity[:]...)
	}
	for index := byte(1); index <= 2; index++ {
		record := make([]byte, 108)
		record[0], record[32], record[64] = index, index, 4
		binary.BigEndian.PutUint64(record[96:104], uint64(window.Unix()))
		record[104], record[106], record[107] = 1, 1, 1 // class, count=1, committed
		legacy = append(legacy, record...)
		promoted = append(promoted, record[:105]...)
		promoted = append(promoted, 1)
		promoted = append(promoted, record[105:]...)
	}
	return legacy, promoted, window
}

func checkPromotedLegacyQuota(t *testing.T, root string, window time.Time) {
	t.Helper()
	network, node, profile := [32]byte{1}, [32]byte{2}, [32]byte{3}
	ledger, err := openClosedTokenIssuerLedger(root, network, node, profile)
	if err != nil {
		t.Fatal(err)
	}
	_, expected, _ := legacyPromotionBytes()
	raw, err := os.ReadFile(filepath.Join(root, "closed-token-ledger"))
	if err != nil || !bytes.Equal(raw, expected) {
		t.Fatalf("healthy recovery changed the independent complete history: %v", err)
	}
	request := admissiontoken.ClosedTokenBatchRequest{
		Permission: admission.Permission{PermissionID: [32]byte{4}, Maxima: [3]uint32{3, 0, 0}},
		Class:      1, WindowStart: window, BlindedRequests: [][]byte{{1}},
	}
	for index := byte(1); index <= 2; index++ {
		request.RequestID = [32]byte{index}
		if ok, err := ledger.reserve(request, request.RequestID, closedIssuanceBootstrap); err != nil || !ok {
			t.Fatalf("retained exact retry %d: %t / %v", index, ok, err)
		}
		if ok, err := ledger.reserve(request, request.RequestID, closedIssuanceAdmitted); err == nil || ok {
			t.Fatalf("retained retry changed kind: %t / %v", ok, err)
		}
	}
	request.RequestID = [32]byte{3}
	if ok, err := ledger.reserve(request, request.RequestID, closedIssuanceBootstrap); err != nil || ok {
		t.Fatalf("promotion refunded bootstrap allowance: %t / %v", ok, err)
	}
	if ok, err := ledger.reserve(request, request.RequestID, closedIssuanceAdmitted); err != nil || !ok {
		t.Fatalf("remaining admitted allowance: %t / %v", ok, err)
	}
	ledger, err = openClosedTokenIssuerLedger(root, network, node, profile)
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = [32]byte{4}
	if ok, err := ledger.reserve(request, request.RequestID, closedIssuanceAdmitted); err != nil || ok {
		t.Fatalf("reopen refunded total allowance: %t / %v", ok, err)
	}
}
