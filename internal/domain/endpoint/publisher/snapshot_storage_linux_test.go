package publisher

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
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
	principal := [32]byte{2}
	local, err := executionruntime.New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: principal, Surface: execution.Administration}}})
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	owner := &Publisher{ctx: t.Context(), boundary: func() error { return nil }, local: local, principal: [32]byte{3}}
	refusedReceive := func(context.Context, io.Writer) error {
		t.Error("refused reservation called receive")
		return nil
	}
	if err := owner.StageSnapshot(t.Context(), storage, 2, refusedReceive); err == nil || storage.busy.Load() {
		t.Fatalf("foreign Principal reserved storage: %v", err)
	}
	owner.principal = principal
	if err := owner.StageSnapshot(t.Context(), storage, 3, refusedReceive); !errors.Is(err, errSnapshotStage) {
		t.Fatalf("over-quota staging = %v", err)
	}

	aborted := errors.New("client aborted after sending input")
	err = owner.StageSnapshot(t.Context(), storage, 2, func(_ context.Context, output io.Writer) error {
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
	if err := owner.StageSnapshot(t.Context(), storage, 2, func(context.Context, io.Writer) error { return aborted }); !errors.Is(err, aborted) {
		t.Fatalf("reservation was not reusable after joined abort: %v", err)
	}
	if err := owner.StageSnapshot(t.Context(), storage, 2, func(_ context.Context, output io.Writer) error {
		_, err := output.Write([]byte{0x00, 0xff})
		return err
	}); err == nil {
		t.Fatal("unqualified Publisher accepted a confirmed snapshot")
	}
	if err := owner.StageSnapshot(t.Context(), storage, 2, func(context.Context, io.Writer) error { return aborted }); !errors.Is(err, aborted) {
		t.Fatalf("failed Publisher handoff retained the reservation: %v", err)
	}
}
