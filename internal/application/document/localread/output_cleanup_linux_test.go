//go:build linux

package localread_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/document/localread"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

type presentationOutput struct {
	bytes.Buffer
	closed bool
	err    error
}

func (output *presentationOutput) Close() error { output.closed = true; return output.err }

func TestDocumentReadPreservesPresentationCleanupFailure(t *testing.T) {
	peer := &documentReadPeer{response: []byte("ok"), class: endpointstream.CleanClose}
	socket := documentReadSocket(t, peer)
	output := &presentationOutput{err: io.ErrClosedPipe}
	if err := localread.Run(t.Context(), socket, io.NopCloser(strings.NewReader("fixture-link\n")), output); !errors.Is(err, io.ErrClosedPipe) || !output.closed {
		t.Fatal("UI reported success after presentation close failed")
	}
}
