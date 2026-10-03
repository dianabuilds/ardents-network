package issuer

import (
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"
)

func TestIssuerRetainedLedgerRequiresDurabilityBarrier(t *testing.T) {
	root := t.TempDir()
	network, node, profile := [32]byte{1}, [32]byte{2}, [32]byte{3}
	ledger, err := openClosedTokenIssuerLedger(root, network, node, profile)
	if err != nil {
		t.Fatal(err)
	}
	request := admissiontoken.ClosedTokenBatchRequest{Permission: admission.Permission{PermissionID: [32]byte{4}, Maxima: [3]uint32{1, 0, 0}},
		RequestID: [32]byte{5}, Class: 1, WindowStart: time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour), BlindedRequests: [][]byte{{1}}}
	if ok, err := ledger.reserve(request, [32]byte{6}, closedIssuanceAdmitted); err != nil || !ok {
		t.Fatalf("reserve: %v / %v", ok, err)
	}
	failure := errors.New("durability remains unavailable")
	called := false
	opened, err := openClosedTokenIssuerLedgerWithBarrier(root, network, node, profile, func(path string) error {
		called = true
		if path != root {
			t.Fatal("barrier used another root")
		}
		return failure
	})
	if !called || opened != nil || !errors.Is(err, failure) {
		t.Fatalf("reopen exposed uncertain ledger: %v / %v / %v", called, opened, err)
	}
	opened, err = openClosedTokenIssuerLedger(root, network, node, profile)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := opened.reserve(request, [32]byte{6}, closedIssuanceAdmitted); err != nil || !ok {
		t.Fatalf("healthy exact retry: %v / %v", ok, err)
	}
	request.RequestID[0]++
	if ok, err := opened.reserve(request, [32]byte{7}, closedIssuanceAdmitted); err != nil || ok {
		t.Fatalf("reopen refunded committed quota: %v / %v", ok, err)
	}
}
