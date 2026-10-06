package release

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestAmbiguousMetadataCannotEstablishInitialTrust(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func([]byte) []byte
	}{
		{"duplicate-version", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
		}},
		{"escaped-duplicate", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"version":1`), []byte(`"version":1,"\u0076ersion":1`), 1)
		}},
		{"envelope-case-alias", func(b []byte) []byte { return bytes.Replace(b, []byte(`"signed":`), []byte(`"Signed":`), 1) }},
		{"trailing-object", func(b []byte) []byte { return append(b, []byte(` {}`)...) }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			in := publicVector(t)
			changed := scenario.change(in.RootBytes)
			if bytes.Equal(changed, in.RootBytes) {
				t.Fatal("control did not change input")
			}
			in.RootBytes = changed
			v, err := Open(filepath.Join(t.TempDir(), "history"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := v.Close(); err != nil {
					t.Error(err)
				}
			})
			d := v.Evaluate(context.Background(), in)
			if _, ok := d.Authorization(); ok || d.Outcome != OutcomeReleaseInvalid {
				t.Fatalf("ambiguous metadata accepted: %s %v", d.Outcome, d.Err())
			}
			floors, err := v.CurrentFloors(context.Background())
			if err != nil || floors.RootVersion != 0 {
				t.Fatal("ambiguous metadata established trust", err)
			}
		})
	}
}

func TestAuthenticatedRefusalsRetainRootWithoutAuthorization(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*Inputs)
		want   Outcome
	}{
		{"artifact-substitution", func(in *Inputs) { in.Artifact[0] ^= 1 }, OutcomeReleaseInvalid},
		{"local-platform", func(in *Inputs) { in.Local.Platform = "linux-amd64" }, OutcomeReleaseIncompatible},
		{"expired-reference", func(in *Inputs) { in.Local.RefTime = time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC) }, OutcomeReleaseExpired},
		{"metadata-substitution", func(in *Inputs) { in.Files[metadataBaseURL+"1.targets.json"][20] ^= 1 }, OutcomeReleaseInvalid},
		{"missing-snapshot", func(in *Inputs) { delete(in.Files, metadataBaseURL+"1.snapshot.json") }, OutcomeReleaseInvalid},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			in := publicVector(t)
			scenario.change(&in)
			root := filepath.Join(t.TempDir(), "history")
			v, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			d := v.Evaluate(context.Background(), in)
			if d.Outcome != scenario.want {
				t.Fatalf("want %s got %s cause%v", scenario.want, d.Outcome, d.Err())
			}
			if _, ok := d.Authorization(); ok {
				t.Fatal("refusal minted authorization")
			}
			floors, err := v.CurrentFloors(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if floors.TimestampVersion != 0 || floors.SnapshotVersion != 0 || floors.TargetsVersion != 0 {
				t.Fatal("refused target advanced metadata floors")
			}
			if err = v.Close(); err != nil {
				t.Fatal(err)
			}
			v, err = Open(root)
			if err != nil {
				t.Fatal(err)
			}
			retained, err := v.CurrentFloors(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !floorSetEqual(retained, floors) {
				t.Fatal("refusal lost committed Root on reopen")
			}
			if err = v.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCancelledEvaluationCannotEstablishInitialRoot(t *testing.T) {
	v, err := Open(filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := v.Evaluate(ctx, publicVector(t))
	if !errors.Is(d.Err(), context.Canceled) {
		t.Fatal("original context cause lost")
	}
	if _, ok := d.Authorization(); ok {
		t.Fatal("cancelled evaluation accepted")
	}
	f, err := v.CurrentFloors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if f.RootVersion != 0 {
		t.Fatal("cancelled caller established Root")
	}
	if err = v.Close(); err != nil {
		t.Fatal(err)
	}
}
