package installation

import (
	"bytes"
	"strings"
	"testing"
)

func preparationRecordFixture() preparationRecord {
	return preparationRecord{Schema: "ardents-endpoint-installation-preparation-v1", GenerationDigest: strings.Repeat("01", 32),
		RequestDigest: strings.Repeat("02", 32), Phase: "creating-account"}
}

func TestPreparationRecordsCanonicalBytesAndOrdering(t *testing.T) {
	first := preparationRecordFixture()
	body, err := preparationRecordBytes(first)
	wanted := `{"schema":"ardents-endpoint-installation-preparation-v1","generation_digest":"` + strings.Repeat("01", 32) + `","request_digest":"` + strings.Repeat("02", 32) + `","phase":"creating-account"}` + "\n"
	if err != nil || !bytes.Equal(body, []byte(wanted)) {
		t.Fatalf("canonical record differs: %v", err)
	}
	name, err := preparationNext(preparationRecord{}, first)
	if err != nil || name != "0001.json" {
		t.Fatal("missing initial intent")
	}
	second := first
	second.Phase, second.UID, second.GID = "creating-mutable-roots", 1000, 1001
	third := second
	third.Phase = "mutable-roots-prepared"
	for _, step := range []struct {
		previous, next preparationRecord
		name           string
	}{
		{first, second, "0002.json"}, {second, third, "0003.json"},
	} {
		if name, err := preparationNext(step.previous, step.next); err != nil || name != step.name {
			t.Fatalf("ordered phase refused: %v", err)
		}
	}
	for _, previous := range []preparationRecord{first, second, third} {
		failure := previous
		failure.Phase, failure.OriginalError = "preparation-failed", "original failure"
		if name, err := preparationNext(previous, failure); err != nil || name != "failure.json" {
			t.Fatal("original preparation failure lost")
		}
		if _, err := preparationNext(failure, second); err == nil {
			t.Fatal("failure renewed preparation")
		}
	}
	if _, err := preparationNext(first, third); err == nil {
		t.Fatal("mutable root intent bypassed")
	}
	if _, err := preparationNext(second, second); err == nil {
		t.Fatal("duplicate phase accepted")
	}
	if _, err := preparationNext(third, first); err == nil {
		t.Fatal("completed preparation restarted")
	}
	for _, change := range []func(*preparationRecord){
		func(r *preparationRecord) { r.GenerationDigest = strings.Repeat("03", 32) },
		func(r *preparationRecord) { r.RequestDigest = strings.Repeat("04", 32) },
		func(r *preparationRecord) { r.UID++ },
		func(r *preparationRecord) { r.GID++ },
		func(r *preparationRecord) { r.Schema = "unknown" },
		func(r *preparationRecord) { r.OriginalError = "unexpected success error" },
	} {
		bad := third
		change(&bad)
		if _, err := preparationNext(second, bad); err == nil {
			t.Fatal("changed binding admitted")
		}
	}
}

func TestPreparationRecordsRefuseUnboundedOrMalformedFailure(t *testing.T) {
	for _, diagnostic := range []string{"", strings.Repeat("x", 64<<10), string([]byte{0xff})} {
		r := preparationRecordFixture()
		r.Phase, r.OriginalError = "preparation-failed", diagnostic
		if _, err := preparationRecordBytes(r); err == nil {
			t.Fatal("invalid failure record encoded")
		}
	}
}
