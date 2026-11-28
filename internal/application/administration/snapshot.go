package administration

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"time"
)

const (
	// MaximumSnapshotBytes bounds the in-memory compatibility adapter.
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
	// Publish the terminal response only after the reserved operation joins
	// and releases the admission slot. Receipt permits the next local request.
	response := "unavailable\n"
	defer func() {
		if response != "" {
			writeResponse(connection, response)
		}
	}()
	owner, ok := server.owner.(SnapshotPublisher)
	if !ok && server.stageSnapshot == nil {
		return
	}
	// Ordinary withdrawal remains reachable while staging is in progress.
	select {
	case server.snapshots <- struct{}{}:
		defer func() { <-server.snapshots }()
	default:
		return
	}
	var size [8]byte
	if _, err := io.ReadFull(input, size[:]); err != nil {
		return
	}
	length := binary.BigEndian.Uint64(size[:])
	if length > math.MaxInt64 || server.stageSnapshot == nil && length > MaximumSnapshotBytes {
		return
	}
	// Input keeps its original fifteen-second bound. Publication receives only
	// the remainder of the deadline established before reading the request.
	ctx, cancel := context.WithDeadline(server.ctx, deadline)
	defer cancel()
	if err := localRequestError(ctx, nil); err != nil {
		return
	}
	if err := server.publishSnapshot(ctx, connection, input, int64(length), owner); err != nil {
		if errors.Is(err, errSnapshotUnreadTail) {
			response = ""
		}
		return
	}
	if localRequestError(ctx, nil) == nil {
		response = "published\n"
	}
}

func (server *server) publishSnapshot(ctx context.Context, connection *net.UnixConn, input io.Reader, length int64, owner SnapshotPublisher) error {
	receive := func(receiveContext context.Context, output io.Writer) error {
		interrupted := make(chan struct{})
		stop := context.AfterFunc(receiveContext, func() {
			defer close(interrupted)
			_ = connection.CloseRead()
		})
		defer func() {
			if !stop() {
				<-interrupted
			}
		}()
		if err := receiveContext.Err(); err != nil {
			return err
		}
		if _, err := io.WriteString(connection, snapshotReady); err != nil {
			return err
		}
		if err := receiveSnapshotBody(receiveContext, input, output, length); err != nil {
			return err
		}
		if err := localRequestError(receiveContext, nil); err != nil {
			return err
		}
		deadline, _ := ctx.Deadline()
		return connection.SetWriteDeadline(deadline)
	}
	if server.stageSnapshot != nil {
		return server.stageSnapshot(ctx, length, receive)
	}
	body := bytes.NewBuffer(make([]byte, 0, int(length)))
	defer func() { clear(body.Bytes()) }()
	if err := receive(ctx, body); err != nil {
		return err
	}
	return owner.PublishSnapshot(ctx, body.Bytes())
}

var errSnapshotUnreadTail = errors.New("local Service snapshot surplus exceeds its drain bound")

// receiveSnapshotBody validates the input before the caller can commit its
// staged output. The destination must remain private and provisional.
func receiveSnapshotBody(ctx context.Context, input io.Reader, output io.Writer, length int64) error {
	if err := copySnapshotBody(ctx, input, output, length); err != nil {
		return err
	}
	var ending [1]byte
	if _, err := io.ReadFull(input, ending[:]); err != nil {
		return err
	}
	if ending[0] != snapshotCommit {
		return errors.New("local Service snapshot commit marker is invalid")
	}
	if n, err := input.Read(ending[:]); n != 0 || err != io.EOF {
		if !drainMalformedTail(input, n, err) {
			return errSnapshotUnreadTail
		}
		return errors.New("local Service snapshot has an invalid tail")
	}
	return nil
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
