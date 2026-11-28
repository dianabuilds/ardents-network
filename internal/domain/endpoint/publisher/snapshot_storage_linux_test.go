package publisher

import (
	"errors"
	"io"
	"os"
	"testing"
)

func TestSnapshotStorageReservesBeforeReceiveAndReleasesOnAbort(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	storage, err := OpenSnapshotStorage(t.Context(), root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	owner := &Publisher{ctx: t.Context(), boundary: func() error { return nil }}
	refusedReceive := func(io.Writer) error {
		t.Error("refused reservation called receive")
		return nil
	}
	if err := owner.StageSnapshot(t.Context(), storage, 3, refusedReceive); !errors.Is(err, errSnapshotStage) {
		t.Fatalf("over-quota staging = %v", err)
	}

	aborted := errors.New("client aborted after sending input")
	err = owner.StageSnapshot(t.Context(), storage, 2, func(output io.Writer) error {
		if err := owner.StageSnapshot(t.Context(), storage, 1, refusedReceive); !errors.Is(err, errSnapshotStage) {
			t.Errorf("overlapping reservation = %v", err)
		}
		if _, err := output.Write([]byte{0x00, 0xff}); err != nil {
			return err
		}
		return aborted
	})
	if !errors.Is(err, aborted) {
		t.Fatalf("staging after abort = %v", err)
	}
	if err := owner.StageSnapshot(t.Context(), storage, 2, func(io.Writer) error { return aborted }); !errors.Is(err, aborted) {
		t.Fatalf("reservation was not reusable after joined abort: %v", err)
	}
	if err := owner.StageSnapshot(t.Context(), storage, 2, func(output io.Writer) error {
		_, err := output.Write([]byte{0x00, 0xff})
		return err
	}); err == nil {
		t.Fatal("unqualified Publisher accepted a confirmed snapshot")
	}
	if err := owner.StageSnapshot(t.Context(), storage, 2, func(io.Writer) error { return aborted }); !errors.Is(err, aborted) {
		t.Fatalf("failed Publisher handoff retained the reservation: %v", err)
	}
}
