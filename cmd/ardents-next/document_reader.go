package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/document"
)

var errDocumentInput = errors.New("document destination or local input is invalid")

// readDocument owns the trusted UI's input/output for one explicit read. It sends
// only the typed destination and fixed document request over AAI3; the Endpoint
// remains responsible for canonical Target admission, confinement and Route.
// It neither starts an unconfined worker nor retries another connection.
func readDocument(ctx context.Context, socket string, input io.ReadCloser, output io.WriteCloser) error {
	if ctx == nil || !filepath.IsAbs(socket) || input == nil || output == nil {
		return errDocumentInput
	}
	closeIO := func() error { return errors.Join(input.Close(), output.Close()) }
	return withDocumentPresentation(ctx, closeIO, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		link, err := readDocumentDestination(input)
		if err != nil {
			return err
		}
		return document.PresentLocal(ctx, socket, link, output)
	})
}

func readDocumentDestination(input io.Reader) (string, error) {
	// ReadSlice never allocates beyond this bound for an unterminated line.
	// A line ending is UI framing, not part of the exact destination.
	line, err := bufio.NewReaderSize(input, 514).ReadSlice('\n')
	if err != nil && err != io.EOF {
		return "", errDocumentInput
	}
	line = bytes.TrimSuffix(line, []byte{'\n'})
	line = bytes.TrimSuffix(line, []byte{'\r'})
	request := connection.Request{Destination: connection.TargetLink, Value: string(line)}
	if _, err := connection.EncodeRequest(request); err != nil {
		return "", errDocumentInput
	}
	return request.Value, nil
}
