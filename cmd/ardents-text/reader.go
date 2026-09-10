package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

var errTextInput = errors.New("text destination or local input is invalid")

// readText owns the trusted UI's input/output for one explicit read. It sends
// only the typed destination and fixed text request over AAI3; the Endpoint
// remains responsible for canonical Target admission, confinement and Route.
// It neither starts an unconfined worker nor retries another connection.
func readText(ctx context.Context, socket string, input io.ReadCloser, output io.WriteCloser) (resultErr error) {
	if ctx == nil || !filepath.IsAbs(socket) || input == nil || output == nil {
		return errTextInput
	}
	var closeOnce sync.Once
	var closeErr error
	closeIO := func() {
		closeOnce.Do(func() { closeErr = errors.Join(input.Close(), output.Close()) })
	}
	callbackDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(callbackDone); closeIO() })
	defer func() {
		closeIO()
		if !stop() {
			<-callbackDone
		}
		resultErr = errors.Join(resultErr, closeErr, ctx.Err())
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	link, err := readTextDestination(input)
	if err != nil {
		return err
	}
	// Human input is cancellable. The subsequent single setup/exchange has a
	// finite UI deadline even if the local Endpoint ceases to respond.
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	stream, err := connection.Dial(bounded, socket, connection.Request{Destination: connection.TargetLink, Value: link})
	if err != nil {
		return errors.Join(err, bounded.Err())
	}
	body, err := textdocument.Read(bounded, stream)
	defer clear(body)
	if err != nil {
		return errors.Join(err, bounded.Err())
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return textdocument.WritePlainText(output, body)
}

func readTextDestination(input io.Reader) (string, error) {
	// ReadSlice never allocates beyond this bound for an unterminated line.
	// A line ending is UI framing, not part of the exact destination.
	line, err := bufio.NewReaderSize(input, 514).ReadSlice('\n')
	if err != nil && err != io.EOF {
		return "", errTextInput
	}
	line = bytes.TrimSuffix(line, []byte{'\n'})
	line = bytes.TrimSuffix(line, []byte{'\r'})
	request := connection.Request{Destination: connection.TargetLink, Value: string(line)}
	if _, err := connection.EncodeRequest(request); err != nil {
		return "", errTextInput
	}
	return request.Value, nil
}
