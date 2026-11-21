package enrollment

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCandidateRetainsUntrustedImmutableInventoryWithoutInitialProvenance(t *testing.T) {
	r, files := protectedFixture(t)
	writeFixture(t, &r, files)
	// Candidate bytes need not be the running artifact. Initial verification
	// must continue refusing the exact same input with a different executable.
	r.ExecutablePath = filepath.Join(r.BundleRoot, "1.root.json")
	if _, err := Verify(context.Background(), r); !errors.Is(err, ErrBinding) {
		t.Fatalf("initial program identity: %v", err)
	}
	c, err := ReadCandidate(context.Background(), r.BundleRoot, Headless)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := c.Facts()
	if !ok || !f.Headless || !f.Protected || f.ManifestSHA256 != r.ManifestSHA256 {
		t.Fatalf("candidate declarations: %+v %v", f, ok)
	}
	if reflect.TypeOf(c).ConvertibleTo(reflect.TypeOf(Bundle{})) {
		t.Fatal("candidate can manufacture initial Bundle provenance")
	}
	want := []string{"timestamp.json"}
	if !reflect.DeepEqual(c.MetadataNames(), want) {
		t.Fatalf("candidate projection includes static resources: %v", c.MetadataNames())
	}
	names := c.Names()
	names[0] = "forged"
	metadata := c.MetadataNames()
	metadata[0] = "forged"
	f.Release = "forged"
	data, _ := c.File("ardents-text.conf")
	data[0] ^= 1
	if err := os.WriteFile(filepath.Join(r.BundleRoot, "ardents-text.conf"), []byte("later replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	retained, _ := c.File("ardents-text.conf")
	retainedFacts, _ := c.Facts()
	if !bytes.Equal(retained, files["ardents-text.conf"]) || retainedFacts.Release != "release" ||
		c.Names()[0] == "forged" || !reflect.DeepEqual(c.MetadataNames(), want) {
		t.Fatal("candidate snapshot aliases exported data or later filesystem bytes")
	}
	if _, ok := (Candidate{}).Facts(); ok {
		t.Fatal("zero candidate supplies declarations")
	}
	if raw, ok := (Candidate{}).File("RELEASE"); raw != nil || ok ||
		len((Candidate{}).Names()) != 0 || len((Candidate{}).MetadataNames()) != 0 {
		t.Fatal("zero candidate supplies inventory")
	}
}

func TestCandidateChecksActualBytesAndCompleteInventory(t *testing.T) {
	for _, scenario := range []string{"stale-manifest", "extra", "partial-protected", "partial-headless", "malformed", "retired"} {
		t.Run(scenario, func(t *testing.T) {
			r, files := protectedFixture(t)
			switch scenario {
			case "partial-protected":
				delete(files, "ardents-text-reader.socket")
			case "partial-headless":
				delete(files, "ardents-node-linux-amd64")
			case "retired":
				files["RELEASE"] = bytes.Replace(files["RELEASE"], []byte("enrollment-v3"), []byte("enrollment-v2"), 1)
			}
			writeFixture(t, &r, files)
			var err error
			switch scenario {
			case "stale-manifest":
				err = os.WriteFile(filepath.Join(r.BundleRoot, "timestamp.json"), []byte("changed"), 0o600)
			case "extra":
				err = os.WriteFile(filepath.Join(r.BundleRoot, "foreign"), []byte("extra"), 0o600)
			case "malformed":
				err = os.WriteFile(filepath.Join(r.BundleRoot, "SHA256SUMS"), []byte("malformed\n"), 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			c, err := ReadCandidate(context.Background(), r.BundleRoot, Headless)
			if err == nil {
				t.Fatal("candidate accepted changed or incomplete inventory")
			}
			if scenario == "retired" && !errors.Is(err, ErrLegacyEnrollmentDescriptor) {
				t.Fatalf("retired candidate lost typed refusal: %v", err)
			}
			if _, ok := c.Facts(); ok || len(c.Names()) != 0 {
				t.Fatal("refusal returned partial candidate")
			}
		})
	}
}

func TestCandidateOriginalCancellationAtFinalHandoffRefuses(t *testing.T) {
	r, files := protectedFixture(t)
	writeFixture(t, &r, files)
	observed := &observingContext{Context: context.Background()}
	if _, err := ReadCandidate(observed, r.BundleRoot, Headless); err != nil {
		t.Fatal(err)
	}
	if observed.calls < 3 {
		t.Fatal("did not exercise actual I/O")
	}
	canceled := &observingContext{Context: context.Background(), cancelAt: observed.calls}
	c, err := ReadCandidate(canceled, r.BundleRoot, Headless)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("final cancellation: %v", err)
	}
	if _, ok := c.Facts(); ok {
		t.Fatal("canceled candidate escaped physical cleanup")
	}
}

func TestCandidateInputRefusesWithoutAnySnapshot(t *testing.T) {
	for _, input := range []struct {
		ctx   context.Context
		root  string
		scope Scope
	}{
		{nil, "unread", General},
		{context.Background(), "", General},
		{context.Background(), "unread", Scope(0)},
	} {
		c, err := ReadCandidate(input.ctx, input.root, input.scope)
		if !errors.Is(err, ErrInput) {
			t.Fatalf("invalid request reached physical reads: %v", err)
		}
		if _, ok := c.Facts(); ok {
			t.Fatal("invalid request returned a snapshot")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadCandidate(ctx, "unread", General); !errors.Is(err, context.Canceled) {
		t.Fatalf("original cancellation was replaced by a filesystem error: %v", err)
	}
}
