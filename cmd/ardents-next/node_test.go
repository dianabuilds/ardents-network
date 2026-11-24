package main

import (
	"bytes"
	"context"
	"testing"
)

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
