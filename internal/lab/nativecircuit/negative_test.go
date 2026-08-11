package nativecircuit

import (
	"context"
	"testing"
	"time"
)

func TestFixedNegativeCasesFailClosed(t *testing.T) {
	for _, name := range []string{"wrong-instance", "modified-record", "replay", "wrong-binding", "oversized-frame", "invalid-state"} {
		name := name
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := RunNegative(ctx, name); err != nil {
				t.Fatal(err)
			}
		})
	}
}
