package administration

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"
	"unicode/utf8"
)

const (
	// MaximumSnapshotBytes is the selected immutable text publication limit.
	MaximumSnapshotBytes = 4 << 20
	// SnapshotTimeout bounds the entire Snapshot request, without renewal after input.
	SnapshotTimeout = 45 * time.Second
	snapshotRequest = "snapshot\n"
)

// SnapshotPublisher extends the existing owner-authorized Administration
// boundary with document bytes, never a host path or publication authority.
// PublishSnapshot must authorize and commit the publication before returning
// nil. The input is borrowed only until return; retaining it requires a copy.
// Owners that do not implement this operation refuse it without calling Publish.
type SnapshotPublisher interface {
	PublishSnapshot(context.Context, []byte) error
}

// RequestSnapshot publishes one already imported, stable UTF-8 snapshot. The
// caller must keep its bytes unchanged until return. There is no retry or
// fallback to the bodyless Publish operation.
func RequestSnapshot(ctx context.Context, path string, snapshot []byte) (Outcome, error) {
	if ctx == nil || len(snapshot) > MaximumSnapshotBytes || !utf8.Valid(snapshot) {
		return "", errors.New("local Service snapshot is invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, SnapshotTimeout)
	defer cancel()
	return request(ctx, path, func(output io.Writer) error {
		var header [len(snapshotRequest) + 4]byte
		copy(header[:], snapshotRequest)
		binary.BigEndian.PutUint32(header[len(snapshotRequest):], uint32(len(snapshot)))
		if _, err := output.Write(header[:]); err != nil {
			return err
		}
		_, err := output.Write(snapshot)
		return err
	}, Published)
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
	var size [4]byte
	if _, err := io.ReadFull(input, size[:]); err != nil {
		writeResponse(connection, "unavailable\n")
		return
	}
	length := binary.BigEndian.Uint32(size[:])
	if length > MaximumSnapshotBytes {
		writeResponse(connection, "unavailable\n")
		return
	}
	body := make([]byte, int(length))
	defer clear(body)
	if _, err := io.ReadFull(input, body); err != nil || !utf8.Valid(body) {
		writeResponse(connection, "unavailable\n")
		return
	}
	var trailing [1]byte
	if n, err := input.Read(trailing[:]); n != 0 || err != io.EOF {
		refuseMalformedRequest(connection, input, n, err)
		return
	}
	// Input keeps its original fifteen-second bound. Publication receives only
	// the remainder of the deadline established before reading the request.
	ctx, cancel := context.WithDeadline(server.ctx, deadline)
	defer cancel()
	if err := requestError(ctx, nil); err != nil {
		writeResponse(connection, "unavailable\n")
		return
	}
	if err := connection.SetWriteDeadline(deadline); err != nil {
		return
	}
	if err := owner.PublishSnapshot(ctx, body); err != nil || requestError(ctx, nil) != nil {
		writeResponse(connection, "unavailable\n")
		return
	}
	writeResponse(connection, "published\n")
}
