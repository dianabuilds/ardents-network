package administration

import (
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
	return requestSnapshot(ctx, path, func(output io.Writer) error {
		if err := writeSnapshotHeader(output, int64(len(snapshot))); err != nil {
			return err
		}
		_, err := output.Write(snapshot)
		return err
	})
}

func requestSnapshot(ctx context.Context, path string, stage func(io.Writer) error) (Outcome, error) {
	ctx, cancel := context.WithTimeout(ctx, SnapshotTimeout)
	defer cancel()
	return requestOperation(ctx, path, func(output io.Writer) error {
		if err := stage(output); err != nil {
			return err
		}
		_, err := output.Write([]byte{snapshotCommit})
		return err
	}, Published)
}

func writeSnapshotHeader(output io.Writer, size int64) error {
	var header [len(snapshotRequest) + 8]byte
	copy(header[:], snapshotRequest)
	binary.BigEndian.PutUint64(header[len(snapshotRequest):], uint64(size))
	_, err := output.Write(header[:])
	return err
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
	body := make([]byte, int(length))
	defer clear(body)
	if _, err := io.ReadFull(input, body); err != nil {
		writeResponse(connection, "unavailable\n")
		return
	}
	var ending [1]byte
	if _, err := io.ReadFull(input, ending[:]); err != nil || ending[0] != snapshotCommit {
		writeResponse(connection, "unavailable\n")
		return
	}
	if n, err := input.Read(ending[:]); n != 0 || err != io.EOF {
		refuseMalformedRequest(connection, input, n, err)
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
	if err := connection.SetWriteDeadline(deadline); err != nil {
		return
	}
	if err := owner.PublishSnapshot(ctx, body); err != nil || localRequestError(ctx, nil) != nil {
		writeResponse(connection, "unavailable\n")
		return
	}
	writeResponse(connection, "published\n")
}
