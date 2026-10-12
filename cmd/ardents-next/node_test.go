package main

import (
	"bytes"
	"context"
	"testing"
)

func TestNodeDispatchRequiresExactSelectedOperation(t *testing.T) {
	for _, args := range [][]string{{"node"}, {"node", "source"}, {"node", "issuer-extra"}, {"node", "inspect-process", "extra"}, {"node", "restart"}} {
		var out, diagnostic bytes.Buffer
		if code := run(context.Background(), args, &out, &diagnostic); code != 2 {
			t.Fatalf("unselected Node operation %v returned %d", args, code)
		}
		if out.Len() != 0 {
			t.Fatal("unselected Node operation projected accepting output")
		}
	}
}

func TestCanceledNodeProcessInspectionDoesNotDiscloseNativeCause(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, diagnostic bytes.Buffer
	if code := run(ctx, []string{"node", "inspect-process"}, &out, &diagnostic); code != 1 {
		t.Fatal("canceled process inspection accepted", code)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"outcome":"unavailable"`)) || !bytes.Contains(out.Bytes(), []byte(`"queues":"unmeasured"`)) || !bytes.Contains(out.Bytes(), []byte(`"timers":"unmeasured"`)) {
		t.Fatal("unknown evidence was reported as capacity", out.String())
	}
	if diagnostic.Len() != 0 {
		t.Fatal("inspection disclosed raw native cause")
	}
}
