package runtime

import "testing"

func TestDetachedPreparationGrantsNoPermission(t *testing.T) {
	for _, preparation := range []*Preparation{nil, {}} {
		if preparation.Check() == nil {
			t.Fatal("detached preparation granted authority")
		}
	}
}
