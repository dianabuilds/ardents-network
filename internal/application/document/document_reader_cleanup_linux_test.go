//go:build linux

package document_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/document"
)

type presentationOutput struct {
	bytes.Buffer
	closed bool
	err    error
}

func (output *presentationOutput) Close() error { output.closed = true; return output.err }

func TestDocumentReadPreservesPresentationCleanupFailure(t *testing.T) {
	peer := &documentReadPeer{response: documentResponse([]byte("ok")), class: connection.CleanClose}
	socket := documentReadSocket(t, peer)
	output := &presentationOutput{err: io.ErrClosedPipe}
	if err := document.ReadLocal(t.Context(), socket, io.NopCloser(strings.NewReader("fixture-link\n")), output); !errors.Is(err, io.ErrClosedPipe) || !output.closed {
		t.Fatal("UI reported success after presentation close failed")
	}
}
