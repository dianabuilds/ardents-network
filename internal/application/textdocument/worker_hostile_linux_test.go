//go:build linux && text_worker_hostile

// The qualification artifact uses the same runtime defaults as the fixed worker.
//go:debug containermaxprocs=0
//go:debug updatemaxprocs=0

package textdocument

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// This adversarial artifact is built only by the declared installed tree
// qualification. Root must pin its separate executable digest. No maintained
// product argument or Application message can enable these descendant modes.
func TestMain(m *testing.M) {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "worker-reader", "worker-publisher":
			mode := ReaderWorker
			if os.Args[1] == "worker-publisher" {
				mode = PublisherWorker
			}
			if runHostileTreeWorker(mode) != nil {
				os.Exit(2)
			}
			os.Exit(0)
		case "hostile-child", "hostile-grandchild":
			if runHostileDescendant(os.Args[1] == "hostile-child") != nil {
				os.Exit(2)
			}
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func runHostileTreeWorker(mode WorkerMode) error {
	// Audit inherited authority before Go creates poller or descendant pipes.
	if err := verifyWorkerDescriptors(); err != nil {
		return err
	}
	if err := startHostileDescendant("hostile-child"); err != nil {
		return err
	}
	connection, err := net.FileConn(os.Stdin)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	return RunWorker(ctx, &inheritedWorkerAttachment{Conn: connection}, mode)
}

func startHostileDescendant(mode string) error {
	ready, report, err := os.Pipe()
	if err != nil {
		return err
	}
	defer ready.Close()
	defer report.Close()
	if err := ready.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	command := exec.Command("/ardents-text", mode)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	command.ExtraFiles = []*os.File{report}
	if err := command.Start(); err != nil {
		return err
	}
	// Deliberately do not join or kill descendants: this models a hostile
	// Application, whose complete cgroup the real Endpoint must terminate.
	if err := report.Close(); err != nil {
		return err
	}
	var marker [1]byte
	if _, err := io.ReadFull(ready, marker[:]); err != nil || marker[0] != 1 {
		return errors.New("hostile descendant readiness missing")
	}
	return nil
}

func runHostileDescendant(withGrandchild bool) error {
	signal.Ignore(syscall.SIGTERM, syscall.SIGINT)
	if withGrandchild {
		if err := startHostileDescendant("hostile-grandchild"); err != nil {
			return err
		}
	}
	ready := os.NewFile(3, "locally created descendant readiness pipe")
	if ready == nil {
		return errors.New("hostile descendant readiness descriptor absent")
	}
	if _, err := ready.Write([]byte{1}); err != nil {
		_ = ready.Close()
		return err
	}
	if err := ready.Close(); err != nil {
		return err
	}
	// Keep both inherited attachment descriptions open despite peer EOF.
	// This fallback bound is not the cleanup oracle: the installed test pins
	// live descendants and requires the real two-second cgroup stop to join.
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	<-timer.C
	return errors.New("hostile descendant outlived qualification cleanup")
}
