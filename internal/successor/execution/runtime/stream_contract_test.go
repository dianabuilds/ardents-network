package runtime

import "testing"

func TestDetachedOperationCannotBindWorkloadByteContract(t *testing.T) {
	for _, operation := range []*Operation{nil, {}} {
		if err := operation.CheckReader(); err == nil {
			t.Fatal("missing original invocation supplied Reader authority")
		}
		if _, err := operation.ConnectionContext(); err == nil {
			t.Fatal("missing original invocation supplied local Connection context")
		}
		if _, err := operation.BindStreamContract(512, 4<<20+13); err == nil {
			t.Fatal("missing original operation supplied byte contract")
		}
		if _, _, err := (StreamContract{}).Limits(operation); err == nil {
			t.Fatal("copied limits supplied original authority")
		}
	}
}
