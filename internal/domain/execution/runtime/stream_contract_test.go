package runtime

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
)

func TestDetachedOperationCannotBindWorkloadByteContract(t *testing.T) {
	for _, operation := range []*Operation{nil, {}} {
		if err := operation.CheckReader(); err == nil {
			t.Fatal("missing original invocation supplied Reader authority")
		}
		if _, err := operation.ConnectionContext(); err == nil {
			t.Fatal("missing original invocation supplied local Connection context")
		}
		if _, err := operation.BindStreamContract(1024, 4<<20); err == nil {
			t.Fatal("missing original operation supplied byte contract")
		}
		if _, _, err := (StreamContract{}).Limits(operation); err == nil {
			t.Fatal("copied limits supplied original authority")
		}
	}
}

func TestByteStreamUsesTheFixedSymmetricContract(t *testing.T) {
	limit := connection.ByteStreamDirectionalLimit
	for _, test := range []struct {
		send, receive uint64
		allowed       bool
	}{
		{limit, limit, true},
		{limit, limit + 1, false},
		{limit - 1, limit, false},
		{0, limit, false},
		{qualificationDirectionalLimit, qualificationDirectionalLimit, false},
	} {
		if result := validStreamLimits(true, test.send, test.receive); result != test.allowed {
			t.Errorf("limits %d/%d admitted: %t", test.send, test.receive, result)
		}
	}
	if !validStreamLimits(false, 1024, 4<<20) ||
		!validStreamLimits(false, qualificationDirectionalLimit, qualificationDirectionalLimit) {
		t.Fatal("existing document or qualification limits changed")
	}
}
