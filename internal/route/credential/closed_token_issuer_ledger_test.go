package credential

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClosedTokenIssuerLedgerAllowsOnlyTwoBatchesPerPermission(t *testing.T) {
	network, node, profile := [32]byte{1}, [32]byte{2}, [32]byte{3}
	ledger, err := openClosedTokenIssuerLedger(t.TempDir(), network, node, profile)
	if err != nil {
		t.Fatal(err)
	}
	request := ClosedTokenBatchRequest{Permission: Permission{PermissionID: [32]byte{4}, Maxima: [3]uint32{8, 0, 0}},
		Class: 1, WindowStart: time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour), BlindedRequests: [][]byte{{1}}}
	for index := byte(1); index <= 2; index++ {
		request.RequestID = [32]byte{index}
		reserved, reserveErr := ledger.reserve(request, [32]byte{index + 10})
		if reserveErr != nil || !reserved {
			t.Fatalf("batch %d reserve = %t / %v", index, reserved, reserveErr)
		}
	}
	request.RequestID = [32]byte{3}
	reserved, err := ledger.reserve(request, [32]byte{13})
	if err != nil || reserved {
		t.Fatalf("third bootstrap batch reserve = %t / %v", reserved, err)
	}
}

func TestClosedTokenIssuerLedgerDropsOnlyUncommittedCrashTail(t *testing.T) {
	root := t.TempDir()
	network, node, profile := [32]byte{21}, [32]byte{22}, [32]byte{23}
	ledger, err := openClosedTokenIssuerLedger(root, network, node, profile)
	if err != nil {
		t.Fatal(err)
	}
	request := ClosedTokenBatchRequest{Permission: Permission{PermissionID: [32]byte{24}, Maxima: [3]uint32{2, 0, 0}},
		RequestID: [32]byte{25}, Class: 1, WindowStart: time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour), BlindedRequests: [][]byte{{1}}}
	digest := [32]byte{26}
	if reserved, err := ledger.reserve(request, digest); err != nil || !reserved {
		t.Fatalf("commit reservation = %t / %v", reserved, err)
	}
	tail, err := encodeClosedTokenIssuerReservation(closedTokenIssuerReservation{requestID: [32]byte{27}, requestDigest: [32]byte{28},
		permissionID: request.Permission.PermissionID, window: request.WindowStart, class: 1, count: 1})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, closedTokenIssuerLedgerName)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(tail); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := openClosedTokenIssuerLedger(root, network, node, profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := reopened.find(request.RequestID, digest); err != nil || !found || len(reopened.reservations) != 1 {
		t.Fatalf("recovered ledger = found %t, entries %d, err %v", found, len(reopened.reservations), err)
	}
}
