package command

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestCanceledNodeProcessInspectionDoesNotDiscloseNativeCause(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if code := InspectProcess(ctx, &out); code != 1 {
		t.Fatal("canceled process inspection accepted", code)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"outcome":"unavailable"`)) || !bytes.Contains(out.Bytes(), []byte(`"queues":"unmeasured"`)) || !bytes.Contains(out.Bytes(), []byte(`"timers":"unmeasured"`)) {
		t.Fatal("unknown evidence was reported as capacity", out.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || len(result) != 4 {
		t.Fatal("unavailable inspection exposed unregistered evidence", result, err)
	}
}
