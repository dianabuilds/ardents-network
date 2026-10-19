//go:build linux && text_worker_installed

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/successor/connection"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

func TestInstalledConnectionAuthorityTCP(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, role := range []string{"initiator", "publisher"} {
				t.Run(role, func(t *testing.T) {
					for _, fault := range []string{"worker", "network", "deadline"} {
						t.Run(fault, func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "authority-"+role+"-"+fault) })
					}
				})
			}
		})
	}
}

// Separate actual State roots; only signed public inputs and fixture issuer
// preparation are shared. Closing this local observer cannot revoke receivers.
func installedConnectionLocalNetwork(t *testing.T, original *networkAdmissionFixture) *networkAdmissionFixture {
	local := publicationReceivingNetwork(t, original)
	local.plan, local.spec, local.digest, local.receiver = original.plan, original.spec, original.digest, original.receiver
	return local
}

// Only the actual StopUnit scenarios may retain these role-specific protocol
// failures. Every leaf must be expected; physical cleanup is checked separately.
func onlyInstalledConnectionStoppedWorker(err error, role string) bool {
	if err == nil {
		return false
	}
	if onlyInstalledConnectionCancellation(err) {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !onlyInstalledConnectionStoppedWorker(child, role) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return onlyInstalledConnectionStoppedWorker(wrapped.Unwrap(), role)
	}
	if role == "initiator" {
		return err.Error() == "text reader worker ended or emitted output before Connection transfer"
	}
	return role == "publisher" && (err.Error() == "text Publisher worker exchange was interrupted" ||
		err.Error() == "qualified worker attachment ended unexpectedly")
}

// The observed unique unit selects only the fault target. The original native
// operation is checked again before the existing non-root StopUnit permission
// is exercised. No Job is revoked by the fixture; IPC observes physical loss.
func stopInstalledConnectionWorker(t *testing.T, ctx context.Context, invocation *executionruntime.Invocation, check func() error) {
	t.Helper()
	activation := reflect.ValueOf(invocation).Elem().FieldByName("activation")
	if activation.IsNil() {
		t.Fatal("original activation absent")
	}
	instance := activation.Elem().FieldByName("Instance")
	pid, uid := instance.FieldByName("PID").Uint(), instance.FieldByName("UID").Uint()
	name, role := instance.FieldByName("Name").String(), instance.FieldByName("Role").String()
	suffix := "-" + strconv.Itoa(os.Getpid()) + "-" + strconv.Itoa(os.Geteuid()) + ".service"
	if pid == 0 || uid == 0 || pid == uint64(os.Getpid()) || role != "reader" && role != "publisher" || !strings.HasPrefix(name, "ardents-text-"+role+"@") || !strings.HasSuffix(name, suffix) {
		t.Fatal("original worker process identity differs")
	}
	if err := check(); err != nil {
		t.Fatal("original worker unavailable before fault", err)
	}
	if err := exec.CommandContext(ctx, "systemctl", "--system", "stop", name).Run(); err != nil {
		t.Fatal("original worker stop", err)
	}
}

// Actual checked bytes precede the fault. The second Read holds its completed
// foreign observation while authority is lost; caller memory must stay private.
func assertInstalledConnectionAuthorityLoss(t *testing.T, ctx context.Context, stream, peer *connection.Stream,
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
