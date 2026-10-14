package administration

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type snapshotTestOwner struct {
	testInterface
	snapshot func(context.Context, []byte) error
}

func (owner snapshotTestOwner) PublishSnapshot(ctx context.Context, body []byte) error {
	return owner.snapshot(ctx, body)
}
func snapshotTestServer(t *testing.T, owner Interface) (string, Server) {
	t.Helper()
	path := filepath.Join(os.TempDir(), fmt.Sprintf("as-%d.sock", time.Now().UnixNano()))
	server, err := Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return path, server
}
func TestSnapshotAdministrationTransfersOnlyBoundedContent(t *testing.T) {
	for _, size := range []int{0, 65536, MaximumSnapshotBytes} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			body := bytes.Repeat([]byte("x"), size)
			var calls atomic.Int32
			path, _ := snapshotTestServer(t, snapshotTestOwner{
				snapshot: func(_ context.Context, got []byte) error {
					calls.Add(1)
					if !bytes.Equal(got, body) {
						return errors.New("snapshot differs")
					}
					return nil
				},
			})
			outcome, err := RequestSnapshot(t.Context(), path, body)
			if err != nil || outcome != Published || calls.Load() != 1 {
				t.Fatalf("publication = %q, %v, calls %d", outcome, err, calls.Load())
			}
		})
	}
}
func TestSnapshotAdministrationNeverFallsBackToBodylessPublish(t *testing.T) {
	var calls atomic.Int32
	path, _ := snapshotTestServer(t, testInterface{publish: func(context.Context) error { calls.Add(1); return nil }})
	if _, err := RequestSnapshot(t.Context(), path, []byte("private document")); err == nil || calls.Load() != 0 {
		t.Fatalf("old owner accepted snapshot or bodyless fallback, calls %d", calls.Load())
	}
}
func TestSnapshotAdministrationRefusesMalformedWireBeforeOwner(t *testing.T) {
	var calls atomic.Int32
	path, _ := snapshotTestServer(t, snapshotTestOwner{snapshot: func(context.Context, []byte) error { calls.Add(1); return nil }})
	frame := func(length uint32, body []byte) []byte {
		raw := append([]byte(snapshotRequest), binary.BigEndian.AppendUint32(nil, length)...)
		return append(raw, body...)
	}
	for _, raw := range [][]byte{
		frame(MaximumSnapshotBytes+1, nil), frame(1, nil), frame(0, []byte("extra")),
		frame(1, []byte{0xff}), []byte(snapshotRequest + "short"),
	} {
		socket, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
		if err != nil {
			t.Fatal(err)
		}
		_ = socket.SetDeadline(time.Now().Add(time.Second))
		_, _ = socket.Write(raw)
		_ = socket.CloseWrite()
		response := make([]byte, len("unavailable\n"))
		_, err = io.ReadFull(socket, response)
		_ = socket.Close()
		if err != nil || string(response) != "unavailable\n" {
			t.Fatalf("refusal = %q, %v", response, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("malformed snapshot reached publication owner")
	}
}
func TestSnapshotAdministrationSerializesAllocationAndJoinsShutdown(t *testing.T) {
	entered := make(chan struct{})
	joined := make(chan struct{})
	path, server := snapshotTestServer(t, snapshotTestOwner{
		testInterface: testInterface{withdraw: func(context.Context) error { return nil }},
		snapshot: func(ctx context.Context, body []byte) error {
			close(entered)
			<-ctx.Done()
			close(joined)
			return ctx.Err()
		},
	})
	completed := make(chan error, 1)
	go func() { _, err := RequestSnapshot(t.Context(), path, []byte("snapshot")); completed <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("owner did not start")
	}
	if _, err := RequestSnapshot(t.Context(), path, []byte("second")); err == nil {
		t.Fatal("overlapping snapshot accepted")
	}
	if outcome, err := Request(t.Context(), path, Withdraw); err != nil || outcome != Withdrawn {
		t.Fatalf("withdrawal was blocked by snapshot: %q, %v", outcome, err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-joined:
	default:
		t.Fatal("shutdown did not join snapshot owner")
	}
	select {
	case err := <-completed:
		if err == nil {
			t.Fatal("cancelled publication reported success")
		}
	case <-time.After(time.Second):
		t.Fatal("client did not terminate")
	}
}
func TestSnapshotClientRejectsInvalidInputAndWrongSuccess(t *testing.T) {
	for _, invalid := range []struct {
		name string
		ctx  context.Context
		body []byte
	}{
		{"nil-context", nil, nil},
		{"invalid-utf8", t.Context(), []byte{0xff}},
		{"oversized", t.Context(), make([]byte, MaximumSnapshotBytes+1)},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			if _, err := RequestSnapshot(invalid.ctx, "", invalid.body); err == nil {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
	path, _ := snapshotTestServer(t, snapshotTestOwner{snapshot: func(context.Context, []byte) error { return errors.New("not committed") }})
	if _, err := RequestSnapshot(t.Context(), path, nil); err == nil {
		t.Fatal("owner failure reported success")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := RequestSnapshot(ctx, path, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request: %v", err)
	}
}

func TestSnapshotAdministrationRetainsOriginalDeadline(t *testing.T) {
	for _, condition := range []string{"original", "earlier-parent", "expired", "late-success"} {
		t.Run(condition, func(t *testing.T) {
			path := filepath.Join(os.TempDir(), fmt.Sprintf("as-bound-%d.sock", time.Now().UnixNano()))
			listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			client, err := net.DialUnix("unix", nil, listener.Addr().(*net.UnixAddr))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			connection, err := listener.AcceptUnix()
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			_ = client.SetDeadline(time.Now().Add(time.Second))
			// Model time already spent on input: the handler must not restart 45s.
			deadline := time.Now().Add(23 * time.Second)
			parent, cancel := context.WithCancel(t.Context())
			defer cancel()
			wantDeadline := deadline
			if condition == "earlier-parent" {
				wantDeadline = deadline.Add(-time.Second)
				var stop context.CancelFunc
				parent, stop = context.WithDeadline(parent, wantDeadline)
				defer stop()
			}
			if condition == "expired" {
				deadline = time.Now().Add(-time.Second)
			}
			calls := 0
			owner := &server{ctx: parent, snapshots: make(chan struct{}, 1), owner: snapshotTestOwner{
				snapshot: func(ctx context.Context, _ []byte) error {
					calls++
					if got, ok := ctx.Deadline(); !ok || !got.Equal(wantDeadline) {
						t.Errorf("owner deadline = %s, %v; want original %s", got, ok, wantDeadline)
					}
					if condition == "late-success" {
						cancel()
					}
					return nil
				},
			}}
			owner.handleSnapshot(connection, bytes.NewReader([]byte{0, 0, 0, 0}), deadline)
			response, err := io.ReadAll(client)
			want, wantCalls := "published\n", 1
			if condition == "expired" || condition == "late-success" {
				want = "unavailable\n"
			}
			if condition == "expired" {
				wantCalls = 0
			}
			if err != nil || string(response) != want || calls != wantCalls {
				t.Fatalf("response = %q, %v; calls %d; want %q, calls %d", response, err, calls, want, wantCalls)
			}
		})
	}
}

func TestSnapshotClientPreservesEarlierDeadline(t *testing.T) {
	entered := make(chan struct{})
	path, _ := snapshotTestServer(t, snapshotTestOwner{snapshot: func(ctx context.Context, _ []byte) error {
		close(entered)
		<-ctx.Done()
		return ctx.Err()
	}})
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	if _, err := RequestSnapshot(ctx, path, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("earlier caller deadline = %v", err)
	}
	select {
	case <-entered:
	default:
		t.Fatal("request did not reach owner before caller deadline")
	}
}
