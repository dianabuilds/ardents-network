package server

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

// ServeStream owns the established local framing and joins its reader.
// Local callers must authorize the stream before passing it here.
func ServeStream(ctx context.Context, local *net.UnixConn, application endpointstream.Stream, directionalEOF bool) (result error) {
	stop := context.AfterFunc(ctx, func() { _ = application.Close() })
	defer stop()
	defer func() { result = errors.Join(result, application.Close()) }()
	if _, err := local.Write([]byte{1}); err != nil {
		return
	}
	inputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		if readFrames(local, application) != nil {
			_ = application.Close()
		}
	}()
	defer func() { _ = application.Close(); _ = local.Close(); <-inputDone }()
	buffer := make([]byte, endpointstream.MaximumFrame)
	for {
		read, readErr := application.Read(buffer)
		if read > 0 && endpointstream.WriteData(local, buffer[:read]) != nil {
			return
		}
		if readErr != nil {
			if directionalEOF && errors.Is(readErr, io.EOF) {
				if _, err := local.Write([]byte{0, 0, 0, 0}); err != nil {
					return
				}
			}
			break
		}
	}
	var outcome endpointstream.Outcome
	select {
	case value, open := <-application.Done():
		if open {
			outcome = value
		}
	case <-ctx.Done():
		outcome = endpointstream.Outcome{Class: endpointstream.LocalTimeout, Reason: "Endpoint stopped"}
	}
	if outcome.Class == "" {
		outcome = endpointstream.Outcome{Class: endpointstream.IndeterminateFailure, Reason: "Application Connection ended without an outcome"}
	}
	if err := application.Close(); err != nil && (!directionalEOF || outcome.Class == endpointstream.CleanClose) {
		outcome = endpointstream.Outcome{Class: endpointstream.IndeterminateFailure, Reason: "Application cleanup did not complete"}
	}
	_ = endpointstream.WriteTerminal(local, outcome)
	return result
}

func readFrames(reader io.Reader, application interface {
	io.Writer
	CloseInput() error
}) error {
	var header [4]byte
	for {
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			return err
		}
		length := binary.BigEndian.Uint32(header[:])
		if length == 0 {
			return application.CloseInput()
		}
		if length > endpointstream.MaximumFrame {
			return errors.New("local Application frame exceeds its bound")
		}
		data := make([]byte, int(length))
		if _, err := io.ReadFull(reader, data); err != nil {
			return err
		}
		if _, err := application.Write(data); err != nil {
			return err
		}
	}
}

// WriteSetupRefusal writes the refused setup marker and terminal outcome.
func WriteSetupRefusal(writer io.Writer, cause error) error {
	if _, err := writer.Write([]byte{0}); err != nil {
		return err
	}
	return endpointstream.WriteTerminal(writer, classifySetupRefusal(cause))
}
