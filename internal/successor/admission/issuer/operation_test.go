package issuer

import "github.com/dianabuilds/ardents-network/internal/successor/admission/quota"

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
)

func TestOperationRefusesBeforeOpeningInvalidRoots(t *testing.T) {
	root := t.TempDir()
	p := Plan{AdmissionRoot: filepath.Join(root, "a"), KeyRoot: filepath.Join(root, "k"), ResultRoot: filepath.Join(root, "r")}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r := Initialize(ctx, p)
	want := "canceled"
	if !issuance.Supported() {
		want = "unsupported-platform"
	}
	if r.Outcome != want || len(r.Response) != 0 {
		t.Fatal(r)
	}
	p.KeyRoot = p.AdmissionRoot
	p.ResultRoot = filepath.Join(p.AdmissionRoot, "nested")
	r = Issue(t.Context(), p, nil, admission.Facts{}, quota.Bootstrap)
	want = "invalid-input"
	if !issuance.Supported() {
		want = "unsupported-platform"
	}
	if r.Outcome != want {
		t.Fatal(r)
	}
	if category(errors.Join(context.Canceled, issuance.ErrUncertain)) != "storage-uncertain" {
		t.Fatal("cleanup/commit error masked")
	}
}
