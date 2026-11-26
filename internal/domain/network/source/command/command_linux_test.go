package command

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source/serving"
)

func TestDispatchRefusesInvalidOrCanceledSourceBeforeEffects(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "missing-plan")
	output := filepath.Join(root, "output")
	var out, diagnostic bytes.Buffer
	for _, args := range [][]string{nil, {"serve"}, {"prepare-state", input}, {"prepare-trust", input}} {
		if code := Run(t.Context(), args, &out, &diagnostic); code != 2 {
			t.Fatalf("%v returned %d", args, code)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if code := Run(ctx, []string{"prepare-trust", input, output}, &out, &diagnostic); code != 130 {
		t.Fatal("canceled prepare-trust returned", code)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 || out.Len() != 0 || diagnostic.Len() != 0 {
		t.Fatal("refusal produced effects", entries, err, out.String(), diagnostic.String())
	}
}

func TestListeningUsesOnlyReadyOutput(t *testing.T) {
	var out, diagnostic bytes.Buffer
	err := Events(&out, &diagnostic)(t.Context(), serving.Event{Phase: "listening", Outcome: "original-source"})
	if want := "{\"operation\":\"source.serve\",\"phase\":\"listening\",\"outcome\":\"original-source\"}\n"; err != nil || out.String() != want || diagnostic.Len() != 0 {
		t.Fatal("original Source report changed", err, out.String())
	}
}

func TestSourcePreparationRefusesOtherPrincipalBeforeInputEffects(t *testing.T) {
	if os.Geteuid() >= 100 && os.Geteuid() <= 999 {
		t.Fatal("ordinary test profile requires an unprivileged development user outside the fixed system-account range")
	}
	var out, diagnostic bytes.Buffer
	if code := Run(t.Context(), []string{"prepare-state", "/unopened-plan", "/unopened-bundle"}, &out, &diagnostic); code == 0 || out.Len() != 0 {
		t.Fatal("foreign principal reached preparation or receipt", code)
	}
}
