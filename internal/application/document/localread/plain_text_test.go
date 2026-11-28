package localread

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/document"
)

func TestPlainTextEscapesControlsBeforePresentation(t *testing.T) {
	body := []byte("text\n\t\x00\x08\x0d\x1b\x7f\u0085\u009b\u061c\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069 https://example.invalid/ Привет")
	want := "text\n\t\\u0000\\u0008\\u000d\\u001b\\u007f\\u0085\\u009b\\u061c\\u200e\\u200f\\u202a\\u202b\\u202c\\u202d\\u202e\\u2066\\u2067\\u2068\\u2069 https://example.invalid/ Привет"
	var output bytes.Buffer
	if err := writePlainText(&output, body); err != nil || output.String() != want {
		t.Fatalf("safe presentation = %q, %v", output.String(), err)
	}
}

func TestPlainTextRejectsInvalidCompleteBodyBeforeOutput(t *testing.T) {
	for _, body := range [][]byte{{0xff}, []byte("valid prefix\xff"), bytes.Repeat([]byte("x"), document.MaximumBytes+1)} {
		var output bytes.Buffer
		if err := writePlainText(&output, body); err == nil || output.Len() != 0 {
			t.Fatal("invalid body reached presentation")
		}
	}
	if err := writePlainText(nil, nil); err == nil {
		t.Fatal("missing presentation output accepted")
	}
}

func TestPlainTextBoundsExpansionAndReportsOutputFailure(t *testing.T) {
	body := bytes.Repeat([]byte{0}, document.MaximumBytes)
	var output bytes.Buffer
	if err := writePlainText(&output, body); err != nil || output.Len() != 6*document.MaximumBytes ||
		strings.Trim(output.String(), "\\u0") != "" {
		t.Fatal("maximum control-only document was not bounded and escaped")
	}
	if err := writePlainText(rejectPresentation{}, []byte("ok")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("presentation lost output failure")
	}
}

type rejectPresentation struct{}

func (rejectPresentation) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
