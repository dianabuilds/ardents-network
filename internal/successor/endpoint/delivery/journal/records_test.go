package journal

import (
	"bytes"
	"strings"
	"testing"
)

func preparationRecordFixture() Record {
	return Record{Schema: "ardents-endpoint-installation-preparation-v1", GenerationDigest: strings.Repeat("01", 32),
		RequestDigest: strings.Repeat("02", 32), Phase: "creating-account"}
}

func TestPreparationRecordsCanonicalBytesAndOrdering(t *testing.T) {
	first := preparationRecordFixture()
	body, err := Bytes(first)
	wanted := `{"schema":"ardents-endpoint-installation-preparation-v1","generation_digest":"` + strings.Repeat("01", 32) + `","request_digest":"` + strings.Repeat("02", 32) + `","phase":"creating-account"}` + "\n"
	if err != nil || !bytes.Equal(body, []byte(wanted)) {
		t.Fatalf("canonical record differs: %v", err)
	}
	name, err := Next(Record{}, first)
	if err != nil || name != "0001.json" {
		t.Fatal("missing initial intent")
	}
	second := first
	second.Phase, second.UID, second.GID = "creating-mutable-roots", 1000, 1001
	third := second
	third.Phase = "mutable-roots-prepared"
	for _, step := range []struct {
		previous, next Record
		name           string
	}{
		{first, second, "0002.json"}, {second, third, "0003.json"},
	} {
		if name, err := Next(step.previous, step.next); err != nil || name != step.name {
			t.Fatalf("ordered phase refused: %v", err)
		}
	}
	for _, previous := range []Record{first, second, third} {
		failure := previous
		failure.Phase, failure.OriginalError = "preparation-failed", "original failure"
		if name, err := Next(previous, failure); err != nil || name != "failure.json" {
			t.Fatal("original preparation failure lost")
		}
		if _, err := Next(failure, second); err == nil {
			t.Fatal("failure renewed preparation")
		}
	}
	if _, err := Next(first, third); err == nil {
		t.Fatal("mutable root intent bypassed")
	}
	if _, err := Next(second, second); err == nil {
		t.Fatal("duplicate phase accepted")
	}
	if _, err := Next(third, first); err == nil {
		t.Fatal("completed preparation restarted")
	}
	for _, change := range []func(*Record){
		func(r *Record) { r.GenerationDigest = strings.Repeat("03", 32) },
		func(r *Record) { r.RequestDigest = strings.Repeat("04", 32) },
		func(r *Record) { r.UID++ },
		func(r *Record) { r.GID++ },
		func(r *Record) { r.Schema = "unknown" },
		func(r *Record) { r.OriginalError = "unexpected success error" },
	} {
		bad := third
		change(&bad)
		if _, err := Next(second, bad); err == nil {
			t.Fatal("changed binding admitted")
		}
	}
}

func TestPreparationRecordsRefuseUnboundedOrMalformedFailure(t *testing.T) {
	for _, diagnostic := range []string{"", strings.Repeat("x", 64<<10), string([]byte{0xff})} {
		r := preparationRecordFixture()
		r.Phase, r.OriginalError = "preparation-failed", diagnostic
		if _, err := Bytes(r); err == nil {
			t.Fatal("invalid failure record encoded")
		}
	}
}
