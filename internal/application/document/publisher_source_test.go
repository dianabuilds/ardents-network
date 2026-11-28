package document_test

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

type observedSnapshotSource struct {
	*bytes.Reader
	maximumRead int
	starts      int
}

func (source *observedSnapshotSource) ReadAt(body []byte, offset int64) (int, error) {
	source.maximumRead = max(source.maximumRead, len(body))
	if offset == 0 {
		source.starts++
	}
	return source.Reader.ReadAt(body, offset)
}

func TestPublisherSnapshotSourceStreamsIndependentBoundedReads(t *testing.T) {
	for _, size := range []int{0, 160<<10 + 1, document.MaximumBytes + 17} {
		body := bytes.Repeat([]byte{0x80, 0xff, 0x00, 0x4d}, (size+3)/4)[:size]
		source := &observedSnapshotSource{Reader: bytes.NewReader(body)}
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		endpoint, worker := net.Pipe()
		workerDone := make(chan error, 1)
		go func() { workerDone <- document.RunWorker(ctx, worker, document.PublisherWorker) }()
		harness := &publisherHarness{incoming: make(chan endpointstream.Stream), finished: make(chan struct{}), cancel: cancel}
		var serving bool
		t.Cleanup(func() {
			cancel()
			_ = endpoint.Close()
			_ = worker.Close()
			if serving {
				<-harness.finished
			}
			<-workerDone
		})
		if err := document.InitializeSnapshotWorker(ctx, endpoint, [32]byte{17}); err != nil {
			t.Fatal(err)
		}
		if source.starts != 0 {
			t.Fatal("worker initialization read snapshot contents")
		}
		serving = true
		go func() {
			defer close(harness.finished)
			harness.err = document.ServeWorkerSnapshot(ctx, endpoint, harness.incoming, source, int64(size))
		}()
		first, second := newPublisherFixture(nil, nil), newPublisherFixture(nil, nil)
		invalid := newPublisherFixture([]byte{0x01}, nil)
		harness.admit(t, first)
		harness.admit(t, invalid)
		harness.admit(t, second)
		harness.drain(t)
		first.assertResponse(t, body)
		second.assertResponse(t, body)
		invalid.assertResponse(t, nil)
		if size > 0 && (source.maximumRead != 16<<10 || source.starts != 2) {
			t.Fatalf("source reads = largest %d, %d initial positions", source.maximumRead, source.starts)
		}
	}
}
