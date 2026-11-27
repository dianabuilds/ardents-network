package endpointstream

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
)

// ServeLocalStream owns the established local framing and joins its reader.
// Local callers must authorize the stream before passing it here.
func ServeLocalStream(ctx context.Context, local *net.UnixConn, application Stream, directionalEOF bool) (result error) {
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
	buffer := make([]byte, maximumFrame)
	for {
		read, readErr := application.Read(buffer)
		if read > 0 && writeData(local, buffer[:read]) != nil {
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
	var outcome Outcome
	select {
	case value, open := <-application.Done():
		if open {
			outcome = value
		}
	case <-ctx.Done():
		outcome = Outcome{Class: LocalTimeout, Reason: "Endpoint stopped"}
	}
	if outcome.Class == "" {
		outcome = Outcome{Class: IndeterminateFailure, Reason: "Application Connection ended without an outcome"}
	}
	if err := application.Close(); err != nil && (!directionalEOF || outcome.Class == CleanClose) {
		outcome = Outcome{Class: IndeterminateFailure, Reason: "Application cleanup did not complete"}
	}
	_ = writeTerminal(local, outcome)
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
		if length > maximumFrame {
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

func writeTerminal(writer io.Writer, outcome Outcome) error {
	if err := validOutcome(outcome); err != nil {
		return err
	}
	var header [8]byte
	binary.BigEndian.PutUint32(header[:4], terminalMarker)
	binary.BigEndian.PutUint16(header[4:6], uint16(len(outcome.Class)))
	binary.BigEndian.PutUint16(header[6:8], uint16(len(outcome.Reason)))
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	if _, err := io.WriteString(writer, string(outcome.Class)); err != nil {
		return err
	}
	_, err := io.WriteString(writer, outcome.Reason)
	return err
}

// WriteLocalStreamRefusal writes the refused setup marker and terminal outcome.
func WriteLocalStreamRefusal(writer io.Writer, cause error) error {
	if _, err := writer.Write([]byte{0}); err != nil {
		return err
	}
	return writeTerminal(writer, classifySetupRefusal(cause))
}
