//go:build linux

package hosting

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"testing"
	"time"
)

const processReservationRoot = "ARDENTS_HOSTING_TEST_RESERVATION_ROOT"

func TestHostingReservationAcrossProcesses(t *testing.T) {
	if root := os.Getenv(processReservationRoot); root != "" {
		holdProcessReservation(t, root)
		return
	}
	for _, episode := range []string{"interrupt", "crash", "uncertain-release"} {
		t.Run(episode, func(t *testing.T) {
			checkProcessReservation(t, episode)
		})
	}
}

func processReservation(now time.Time) ReservationRequest {
	until := now.Add(time.Minute)
	return ReservationRequest{
		Work: Traffic{Tx: 700000000}, Termination: Traffic{Rx: 100000000},
		WorkUntil: until, HoldUntil: until,
	}
}

func holdProcessReservation(t *testing.T, root string) {
	t.Helper()
	ctx, stop := signal.NotifyContext(t.Context(), os.Interrupt)
	defer stop()
	owner, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	}()
	held, err := owner.Reserve(ctx, processReservation(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("reserved")
	<-ctx.Done()
	cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := held.Release(cleanup); err != nil {
		fmt.Println("release refused")
		return
	}
	fmt.Println("released")
}

func checkProcessReservation(t *testing.T, episode string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	root := filepath.Join(t.TempDir(), "budget")
	now := time.Now().UTC().Truncate(time.Second)
	policy := Policy{
		Provider: "process test", Start: now.Add(-time.Minute), End: now.Add(time.Hour),
		Unit: "B", Quantity: 1000000000, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000,
	}
	if err := Initialize(root, policy); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHostingReservationAcrossProcesses$")
	command.Env = append(os.Environ(), processReservationRoot+"="+root)
	out, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	joined := false
	defer func() {
		if !joined {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	scanner := bufio.NewScanner(out)
	if !scanner.Scan() || scanner.Text() != "reserved" {
		t.Fatal("child did not reserve", scanner.Text(), scanner.Err())
	}
	owner, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	snapshot, err := owner.Sample(ctx, time.Second)
	if err != nil || snapshot.Observation().ReservedBytes != 800000000 {
		t.Fatal("child reservation unavailable", snapshot, err)
	}
	if _, err := owner.Reserve(ctx, processReservation(time.Now())); !errors.Is(err, ErrCapacity) {
		t.Fatal("competing reservation", err)
	}

	var committed []byte
	statePath := filepath.Join(root, "budget.json")
	if episode == "uncertain-release" {
		committed, err = os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "budget.pending"), []byte("interrupted"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	if episode == "crash" {
		err = command.Process.Kill()
	} else {
		err = command.Process.Signal(os.Interrupt)
	}
	if err != nil {
		t.Fatal(err)
	}
	var report string
	for scanner.Scan() {
		if scanner.Text() != "PASS" {
			report = scanner.Text()
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	exit := command.Wait()
	joined = true
	switch episode {
	case "crash":
		if exit == nil || command.ProcessState.ExitCode() != -1 {
			t.Fatal("child did not terminate by signal", exit)
		}
	case "interrupt":
		if exit != nil || report != "released" {
			t.Fatal("interrupted child did not release", report, exit)
		}
	case "uncertain-release":
		after, err := os.ReadFile(statePath)
		if exit != nil || report != "release refused" || err != nil || !bytes.Equal(committed, after) {
			t.Fatal("child changed unresolved reservation", report, exit, err)
		}
		if _, err := owner.Observe(ctx); err == nil {
			t.Fatal("unfinished journal admitted observation")
		}
		return
	}

	reopened, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	want := uint64(0)
	if episode == "crash" {
		want = 800000000
	}
	observed, err := reopened.Observe(ctx)
	if err != nil || observed.ReservedBytes != want {
		t.Fatal("unexpected reservation after child exit", observed, want, err)
	}
}
