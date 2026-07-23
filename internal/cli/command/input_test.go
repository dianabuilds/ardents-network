package command

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ardentsv1 "ardents/internal/localapi/protocol"
)

func TestParseFileArgRequiresFileFlag(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseFileArg("publish", &stderr, nil)
	if err == nil || !strings.Contains(err.Error(), "file is required") {
		t.Fatalf("err = %v, want missing file", err)
	}
}

func TestLoadProtoJSONReadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blob.json")
	reference := "bafkreibm6jg3ux5qumhcn2b3flc3tyu6dmlb4xa7u5bf44yegnrjhc4yeq"
	data := []byte(`{"reference":"` + reference + `","state":"available-local","createdAt":"1970-01-01T00:00:01Z"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	msg := &ardentsv1.BlobSnapshot{}
	if err := LoadProtoJSON(nil, path, msg); err != nil {
		t.Fatalf("loadProtoJSON() error = %v", err)
	}
	if msg.GetReference() != reference || msg.GetState() != "available-local" {
		t.Fatalf("msg = %+v", msg)
	}
	if got := msg.GetCreatedAt(); got == nil || !got.AsTime().Equal(time.Unix(1, 0).UTC()) {
		t.Fatalf("created_at not parsed: %+v", got)
	}
}

func TestFirstArgRejectsEmptyInput(t *testing.T) {
	if _, ok := FirstArg(nil); ok {
		t.Fatal("firstArg(nil) unexpectedly returned ok")
	}
	if _, ok := FirstArg([]string{""}); ok {
		t.Fatal("firstArg(empty) unexpectedly returned ok")
	}
	if got, ok := FirstArg([]string{"blob-1"}); !ok || got != "blob-1" {
		t.Fatalf("firstArg() = (%q, %v)", got, ok)
	}
}
