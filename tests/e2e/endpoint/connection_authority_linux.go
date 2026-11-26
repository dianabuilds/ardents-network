//go:build linux && document_worker_installed

package endpoint

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/domain/connection"
)

// Actual checked bytes precede the fault. The second Read holds its completed
// foreign observation while authority is lost; caller memory must stay private.
func AssertInstalledConnectionAuthorityLoss(t *testing.T, ctx context.Context, stream, peer *connection.Stream,
	arm, release func(), entered <-chan struct{}, fault func(), checkJob func() error, workerLoss bool) error {
	t.Helper()
	defer release()
	if n, err := peer.Write([]byte("original bytes")); err != nil || n != len("original bytes") {
		t.Fatal("initial byte admission", n, err)
	}
	first := []byte{0}
	if n, err := stream.Read(first); n != 1 || err != nil || first[0] != 'o' {
		t.Fatal("initial authenticated byte", n, err)
	}
	arm()
	type outcome struct {
		n   int
		err error
	}
	read := make(chan outcome, 1)
	buffer := []byte{0xa5}
	go func() { n, err := stream.Read(buffer); read <- outcome{n, err} }()
	joined := false
	defer func() {
		release()
		if !joined {
			<-read
		}
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("original Read observation absent", ctx.Err())
	}
	fault()
	if workerLoss {
		// The original IPC reader must observe the stopped process. This waits
		// for a real revoke, without revoking the caller on the worker's behalf.
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for checkJob() == nil {
			select {
			case <-timer.C:
				t.Fatal("stopped worker retained operation authority")
			case <-time.After(time.Millisecond):
			}
		}
	} else if err := checkJob(); err != nil {
		t.Fatal("Network/time fault replaced the original Job", err)
	}
	release()
	var got outcome
	select {
	case got = <-read:
		joined = true
	case <-ctx.Done():
		t.Fatal("late Read failed to join", ctx.Err())
	}
	if got.n != 0 || got.err == nil || errors.Is(got.err, io.EOF) || buffer[0] != 0xa5 {
		t.Fatal("authority loss exposed late bytes or EOF", got.n, got.err, buffer[0])
	}
	closeErr := stream.Close()
	if closeErr == nil || stream.JoinedCleanup() != nil {
		t.Fatal("authority refusal lost original outcome or physical join", closeErr, stream.JoinedCleanup())
	}
	if stream.Close() != closeErr {
		t.Fatal("repeated Close changed original outcome")
	}
	for result := range stream.Done() {
		if result.Class != application.IndeterminateFailure {
			t.Fatal("authority loss published successful outcome", result.Class)
		}
	}
	if !workerLoss {
		if err := checkJob(); err != nil {
			t.Fatal("Connection cleanup retired the original Job", err)
		}
	}
	return closeErr
}
