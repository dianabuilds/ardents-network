package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

func TestHostingPlanExactFields(t *testing.T) {
	root := strconv.Quote(filepath.Join(t.TempDir(), "budget"))
	valid := `{"root":` + root + `,"work":{"tx":1,"rx":2},"termination":{"tx":1,"rx":1},"hold_ms":1}`
	if _, err := decodeHostingPlan([]byte(valid), "hold"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{"root":` + root + `,"root":` + root + `}`, `{"root":` + root + `,"extra":1}`, `{"root":null}`,
		`{"root":` + root + `,"work":{"tx":1,"tx":2},"termination":{},"hold_ms":1}`,
		`{"root":` + root + `,"work":{"tx":1,"Tx":2},"termination":{"rx":1},"hold_ms":1}`,
		`{"root":` + root + `,"work":{"tx":1},"termination":{"rx":1,"RX":2},"hold_ms":1}`,
		`{"root":` + root + `,"work":{},"termination":{},"hold_ms":60001}`, valid + ` {}`,
	} {
		if _, err := decodeHostingPlan([]byte(raw), "hold"); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, err := decodeHostingPlan([]byte(`{"root":`+root+`,"policy":{"provider":"one","Provider":"two"}}`), "initialize"); err == nil {
		t.Fatal("case alias in policy accepted")
	}
}

func TestHostingUnsupportedDoesNotReadInput(t *testing.T) {
	if hosting.Supported() {
		return
	} // This behavior belongs to the non-Linux build.
	var out, log bytes.Buffer
	code := run(context.Background(), []string{"hosting", "observe", "--config", "missing"}, &out, &log)
	if code != 1 || !bytes.Contains(out.Bytes(), []byte("unsupported-platform")) || log.Len() != 0 {
		t.Fatalf("%d %s %s", code, &out, &log)
	}
}
