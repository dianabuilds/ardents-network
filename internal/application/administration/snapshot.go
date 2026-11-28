package administration

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"
)

const (
	// MaximumSnapshotBytes is the selected immutable document publication limit.
	MaximumSnapshotBytes = 4 << 20
	// SnapshotTimeout bounds the entire Snapshot request, without renewal after input.
	SnapshotTimeout      = 45 * time.Second
	snapshotRequest      = "stage-v1\n"
	snapshotCommit  byte = 1
	snapshotReady        = "ready\n"
)

// SnapshotPublisher extends the existing owner-authorized Administration
// boundary with document bytes, never a host path or publication authority.
// PublishSnapshot must authorize and commit the publication before returning
// nil. The input is borrowed only until return; retaining it requires a copy.
// Owners that do not implement this operation refuse it without calling Publish.
type SnapshotPublisher interface {
	PublishSnapshot(context.Context, []byte) error
}

// RequestSnapshot publishes one already imported, stable binary snapshot. The
// caller must keep its bytes unchanged until return. There is no retry or
// fallback to the bodyless Publish operation.
func RequestSnapshot(ctx context.Context, path string, snapshot []byte) (Outcome, error) {
	if ctx == nil || len(snapshot) > MaximumSnapshotBytes {
		return "", errors.New("local Service snapshot is invalid")
	}
	return requestSnapshot(ctx, path, func(output io.ReadWriter) error {
		if err := beginSnapshot(output, int64(len(snapshot))); err != nil {
			return err
		}
		_, err := output.Write(snapshot)
		return err
	})
}

func requestSnapshot(ctx context.Context, path string, stage func(io.ReadWriter) error) (Outcome, error) {
	ctx, cancel := context.WithTimeout(ctx, SnapshotTimeout)
	defer cancel()
	return requestOperation(ctx, path, func(output io.ReadWriter) error {
		if err := stage(output); err != nil {
			return err
		}
		_, err := output.Write([]byte{snapshotCommit})
		return err
	}, Published)
}

func beginSnapshot(connection io.ReadWriter, size int64) error {
	var header [len(snapshotRequest) + 8]byte
	copy(header[:], snapshotRequest)
	binary.BigEndian.PutUint64(header[len(snapshotRequest):], uint64(size))
	if _, err := connection.Write(header[:]); err != nil {
		return err
	}
	var response [len(snapshotReady)]byte
	if _, err := io.ReadFull(connection, response[:]); err != nil {
		return err
	}
	if string(response[:]) != snapshotReady {
		return errors.New("local Service snapshot input refused")
	}
	return nil
}

func (server *server) handleSnapshot(connection *net.UnixConn, input io.Reader, deadline time.Time) {
	owner, ok := server.owner.(SnapshotPublisher)
	if !ok {
		writeResponse(connection, "unavailable\n")
		return
	}
	// One snapshot allocation/transition per server. Ordinary withdrawal remains
	// reachable while this operation waits on its publication owner.
	select {
	case server.snapshots <- struct{}{}:
		defer func() { <-server.snapshots }()
	default:
		writeResponse(connection, "unavailable\n")
		return
	}
	var size [8]byte
	if _, err := io.ReadFull(input, size[:]); err != nil {
		writeResponse(connection, "unavailable\n")
		return
	}
	length := binary.BigEndian.Uint64(size[:])
	if length > MaximumSnapshotBytes {
		writeResponse(connection, "unavailable\n")
		return
	}
	// Input keeps its original fifteen-second bound. Publication receives only
	// the remainder of the deadline established before reading the request.
	ctx, cancel := context.WithDeadline(server.ctx, deadline)
	defer cancel()
	if err := localRequestError(ctx, nil); err != nil {
		writeResponse(connection, "unavailable\n")
		return
	}
	body := bytes.NewBuffer(make([]byte, 0, int(length)))
	defer func() { clear(body.Bytes()) }()
	if _, err := io.WriteString(connection, snapshotReady); err != nil {
		return
	}
	if !receiveSnapshotBody(ctx, connection, input, body, int64(length)) {
		return
	}
	if err := localRequestError(ctx, nil); err != nil {
		writeResponse(connection, "unavailable\n")
		return
	}
	if err := connection.SetWriteDeadline(deadline); err != nil {
		return
	}
	if err := owner.PublishSnapshot(ctx, body.Bytes()); err != nil || localRequestError(ctx, nil) != nil {
		writeResponse(connection, "unavailable\n")
		return
	}
	writeResponse(connection, "published\n")
}

// receiveSnapshotBody validates the input before the caller can commit its
// staged output. The destination must remain private and provisional.
func receiveSnapshotBody(ctx context.Context, connection *net.UnixConn, input io.Reader, output io.Writer, length int64) bool {
	if err := copySnapshotBody(ctx, input, output, length); err != nil {
		writeResponse(connection, "unavailable\n")
		return false
	}
	var ending [1]byte
	if _, err := io.ReadFull(input, ending[:]); err != nil || ending[0] != snapshotCommit {
		writeResponse(connection, "unavailable\n")
		return false
	}
	if n, err := input.Read(ending[:]); n != 0 || err != io.EOF {
		refuseMalformedRequest(connection, input, n, err)
		return false
	}
	return true
}

func copySnapshotBody(ctx context.Context, input io.Reader, output io.Writer, length int64) error {
	var scratch [16 << 10]byte
	defer clear(scratch[:])
	for remaining := length; remaining > 0; {
		if err := ctx.Err(); err != nil {
			return err
		}
		block := scratch[:min(int64(len(scratch)), remaining)]
		if _, err := io.ReadFull(input, block); err != nil {
			return err
		}
		n, err := output.Write(block)
		if err != nil {
			return err
		}
		if n != len(block) {
			return io.ErrShortWrite
		}
		remaining -= int64(n)
	}
	return ctx.Err()
}
