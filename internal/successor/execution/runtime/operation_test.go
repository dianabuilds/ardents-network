package runtime

import "testing"

func TestDetachedOperationCannotSupplyPermission(t *testing.T) {
	for _, operation := range []*Operation{nil, {}} {
		if operation.Check() == nil {
			t.Fatal("detached operation supplied permission")
		}
	}
}
