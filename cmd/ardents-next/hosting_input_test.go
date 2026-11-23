package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/hosting"
)

func TestHostingPlanExactFields(t *testing.T) {
	root := strconv.Quote(filepath.Join(t.TempDir(), "budget"))
	valid := `{"root":` + root + `,"policy":{"provider":"private"}}`
	if _, err := decodeHostingPlan([]byte(valid), "initialize"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{"root":` + root + `,"root":` + root + `,"policy":{}}`,
		`{"root":` + root + `,"extra":1,"policy":{}}`,
		`{"root":null,"policy":{}}`,
		`{"root":` + root + `,"policy":{"provider":"one","provider":"two"}}`,
		`{"root":` + root + `,"policy":{"provider":"one","Provider":"two"}}`,
		valid + ` {}`,
	} {
		if _, err := decodeHostingPlan([]byte(raw), "initialize"); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, err := decodeHostingPlan([]byte(`{"root":`+root+`}`), "observe"); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeHostingPlan([]byte(valid), "observe"); err == nil {
		t.Fatal("observation accepted a policy")
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
