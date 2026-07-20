package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ardentsv1 "ardents/proto/ardents/v1"
)

func TestParseFileArgRequiresFileFlag(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseFileArg("publish", &stderr, nil)
	if err == nil || !strings.Contains(err.Error(), "file is required") {
		t.Fatalf("err = %v, want missing file", err)
	}
}

func TestLoadProtoJSONReadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blob.json")
	data := []byte(`{"id":"blob-1","state":"available-local","createdAt":"1970-01-01T00:00:01Z"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	msg := &ardentsv1.BlobSnapshot{}
	if err := loadProtoJSON(path, msg); err != nil {
		t.Fatalf("loadProtoJSON() error = %v", err)
	}
	if msg.GetId() != "blob-1" || msg.GetState() != "available-local" {
		t.Fatalf("msg = %+v", msg)
	}
	if got := msg.GetCreatedAt(); got == nil || !got.AsTime().Equal(time.Unix(1, 0).UTC()) {
		t.Fatalf("created_at not parsed: %+v", got)
	}
}

func TestFirstArgRejectsEmptyInput(t *testing.T) {
	if _, ok := firstArg(nil); ok {
		t.Fatal("firstArg(nil) unexpectedly returned ok")
	}
	if _, ok := firstArg([]string{""}); ok {
		t.Fatal("firstArg(empty) unexpectedly returned ok")
	}
	if got, ok := firstArg([]string{"blob-1"}); !ok || got != "blob-1" {
		t.Fatalf("firstArg() = (%q, %v)", got, ok)
	}
}

func TestFormatStructSortsKeysDeterministically(t *testing.T) {
	got := formatStruct(map[string]any{"b": 2, "a": 1})
	if got != "a=1 b=2" {
		t.Fatalf("formatStruct() = %q", got)
	}
}
