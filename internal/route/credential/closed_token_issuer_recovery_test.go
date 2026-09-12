package credential

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClosedTokenIssuerLedgerKindsShareQuotaAndRetainRetryKind(t *testing.T) {
	root := t.TempDir()
	network, node, profile := [32]byte{1}, [32]byte{2}, [32]byte{3}
	ledger, err := openClosedTokenIssuerLedger(root, network, node, profile)
	if err != nil {
		t.Fatal(err)
	}
	request := ClosedTokenBatchRequest{Permission: Permission{PermissionID: [32]byte{4}, Maxima: [3]uint32{4, 0, 0}},
		Class: 1, WindowStart: time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour), BlindedRequests: [][]byte{{1}}}
	for index, kind := range []closedIssuanceKind{closedIssuanceBootstrap, closedIssuanceAdmitted, closedIssuanceBootstrap} {
		request.RequestID = [32]byte{byte(index + 1)}
		if reserved, err := ledger.reserve(request, request.RequestID, kind); err != nil || !reserved {
			t.Fatalf("reservation %d = %t / %v", index, reserved, err)
		}
	}
	ledger, err = openClosedTokenIssuerLedger(root, network, node, profile)
	if err != nil {
		t.Fatal(err)
	}
	// Exact retries reconcile the original debit; changing channel admission
	// cannot turn that debit into a different kind after restart.
	request.RequestID = [32]byte{2}
	if reserved, err := ledger.reserve(request, request.RequestID, closedIssuanceAdmitted); err != nil || !reserved {
		t.Fatalf("admitted retry = %t / %v", reserved, err)
	}
	if reserved, err := ledger.reserve(request, request.RequestID, closedIssuanceBootstrap); err == nil || reserved {
		t.Fatalf("cross-kind retry = %t / %v", reserved, err)
	}
	request.RequestID = [32]byte{4}
	if reserved, err := ledger.reserve(request, request.RequestID, closedIssuanceBootstrap); err != nil || reserved {
		t.Fatalf("third bootstrap = %t / %v", reserved, err)
	}
	if reserved, err := ledger.reserve(request, request.RequestID, closedIssuanceAdmitted); err != nil || !reserved {
		t.Fatalf("remaining admitted quota = %t / %v", reserved, err)
	}
	request.RequestID = [32]byte{5}
	if reserved, err := ledger.reserve(request, request.RequestID, closedIssuanceAdmitted); err != nil || reserved {
		t.Fatalf("shared permission quota = %t / %v", reserved, err)
	}
}

func TestClosedTokenIssuerLedgerPromotesLegacyWithoutRefund(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "interrupted-promotion"}[interrupted], func(t *testing.T) {
			root := t.TempDir()
			network, node, profile := [32]byte{1}, [32]byte{2}, [32]byte{3}
			owner := &closedTokenIssuerLedger{root: root, network: network, node: node, profileDigest: profile}
			raw := owner.header()
			copy(raw[:8], "ARDILG01")
			window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
			for index := byte(1); index <= 2; index++ {
				record, err := encodeClosedTokenIssuerReservation(closedTokenIssuerReservation{
					requestID: [32]byte{index}, requestDigest: [32]byte{index}, permissionID: [32]byte{4},
					window: window, class: 1, kind: closedIssuanceBootstrap, count: 1})
				if err != nil {
					t.Fatal(err)
				}
				record[len(record)-1] = 1
				// Exact retained v1 format lacks only the kind byte.
				raw = append(raw, record[:105]...)
				raw = append(raw, record[106:]...)
			}
			path := filepath.Join(root, closedTokenIssuerLedgerName)
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if interrupted {
				if err := owner.retainBinding(); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, closedIssuerLedgerStageName), []byte("partial stage"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			ledger, err := openClosedTokenIssuerLedger(root, network, node, profile)
			if err != nil {
				t.Fatal(err)
			}
			for index := byte(1); index <= 2; index++ {
				record, found, err := ledger.find([32]byte{index}, [32]byte{index})
				if err != nil || !found || record.kind != closedIssuanceBootstrap {
					t.Fatalf("legacy debit %d = %+v / %t / %v", index, record, found, err)
				}
			}
			request := ClosedTokenBatchRequest{Permission: Permission{PermissionID: [32]byte{4}, Maxima: [3]uint32{3, 0, 0}},
				RequestID: [32]byte{3}, Class: 1, WindowStart: window, BlindedRequests: [][]byte{{1}}}
			if reserved, err := ledger.reserve(request, request.RequestID, closedIssuanceBootstrap); err != nil || reserved {
				t.Fatalf("legacy bootstrap allowance refunded = %t / %v", reserved, err)
			}
			if reserved, err := ledger.reserve(request, request.RequestID, closedIssuanceAdmitted); err != nil || !reserved {
				t.Fatalf("remaining normal allowance = %t / %v", reserved, err)
			}
			reopened, err := openClosedTokenIssuerLedger(root, network, node, profile)
			if err != nil || len(reopened.reservations) != 3 {
				t.Fatalf("promoted reopen = %v", err)
			}
			request.RequestID = [32]byte{4}
			if reserved, err := reopened.reserve(request, request.RequestID, closedIssuanceAdmitted); err != nil || reserved {
				t.Fatalf("legacy total allowance refunded = %t / %v", reserved, err)
			}
			stored, err := os.ReadFile(path)
			if err != nil || string(stored[:8]) != "ARDILG02" {
				t.Fatalf("promoted format unavailable: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(root, closedIssuerLedgerStageName)); !os.IsNotExist(err) {
				t.Fatalf("promotion stage survived: %v", err)
			}
		})
	}
}

func TestClosedTokenIssuerLedgerMissingRetainedFilesRefusesReset(t *testing.T) {
	for _, name := range []string{closedTokenIssuerLedgerName, closedIssuerLedgerBindingName} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			network, node, profile := [32]byte{1}, [32]byte{2}, [32]byte{3}
			if _, err := openClosedTokenIssuerLedger(root, network, node, profile); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, name)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if _, err := openClosedTokenIssuerLedger(root, network, node, profile); err == nil {
				t.Fatal("missing retained file was accepted")
			}
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("missing retained file was recreated: %v", err)
			}
		})
	}
}

func TestClosedTokenIssuerLedgerDoesNotEraseRecordsAfterUncommittedRecord(t *testing.T) {
	root := t.TempDir()
	network, node, profile := [32]byte{1}, [32]byte{2}, [32]byte{3}
	ledger, err := openClosedTokenIssuerLedger(root, network, node, profile)
	if err != nil {
		t.Fatal(err)
	}
	record, err := encodeClosedTokenIssuerReservation(closedTokenIssuerReservation{
		requestID: [32]byte{1}, requestDigest: [32]byte{2}, permissionID: [32]byte{3},
		window: time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour), class: 1, kind: closedIssuanceBootstrap, count: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range [][]byte{{1}, append([]byte(nil), record...)} {
		raw := append(ledger.header(), record...)
		raw = append(raw, suffix...)
		path := filepath.Join(root, closedTokenIssuerLedgerName)
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := openClosedTokenIssuerLedger(root, network, node, profile); err == nil {
			t.Fatal("nonterminal uncommitted record accepted")
		}
		retained, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(retained, raw) {
			t.Fatalf("refusal modified retained records: %v", err)
		}
	}
}

func TestClosedTokenIssuerLedgerAppendFailureStopsOwner(t *testing.T) {
	root := t.TempDir()
	network, node, profile := [32]byte{1}, [32]byte{2}, [32]byte{3}
	ledger, err := openClosedTokenIssuerLedger(root, network, node, profile)
	if err != nil {
		t.Fatal(err)
	}
	request := ClosedTokenBatchRequest{Permission: Permission{PermissionID: [32]byte{4}, Maxima: [3]uint32{4, 0, 0}},
		RequestID: [32]byte{1}, Class: 1, WindowStart: time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour),
		BlindedRequests: [][]byte{{1}}}
	if reserved, err := ledger.reserve(request, request.RequestID, closedIssuanceBootstrap); err != nil || !reserved {
		t.Fatalf("first reservation = %t / %v", reserved, err)
	}
	path := filepath.Join(root, closedTokenIssuerLedgerName)
	retained := filepath.Join(root, "retained-test-ledger")
	if err := os.Rename(path, retained); err != nil {
		t.Fatal(err)
	}
	request.RequestID = [32]byte{2}
	if reserved, err := ledger.reserve(request, request.RequestID, closedIssuanceAdmitted); err == nil || reserved {
		t.Fatalf("missing append target = %t / %v", reserved, err)
	}
	if err := os.Rename(retained, path); err != nil {
		t.Fatal(err)
	}
	for _, id := range []byte{1, 2, 3} {
		request.RequestID = [32]byte{id}
		if reserved, err := ledger.reserve(request, request.RequestID, closedIssuanceBootstrap); err == nil || reserved {
			t.Fatalf("failed owner resumed request %d = %t / %v", id, reserved, err)
		}
	}
	reopened, err := openClosedTokenIssuerLedger(root, network, node, profile)
	if err != nil || len(reopened.reservations) != 1 {
		t.Fatalf("reopen of intact committed ledger = %v", err)
	}
}
