//go:build linux && text_worker_escape

// The qualification artifact keeps Go from opening ambient cgroup files before
// it audits the only inherited attachment.
//go:debug containermaxprocs=0
//go:debug updatemaxprocs=0

package textdocument

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

const (
	escapeProbeFile = "/tmp/ardents-text-worker-escape-file"
	escapeProbeIPC  = "/tmp/ardents-text-worker-escape.sock"
	escapeProbeTCP4 = "127.0.0.1:45561"
	escapeProbeTCP6 = "[::1]:45562"
	escapeProbeUDP4 = "127.0.0.1:45563"
	escapeProbeDNS  = "127.0.0.1:45564"
)

// TestMain exists only in a separately pinned hostile qualification artifact.
// No product command or Application message can select this execution path.
func TestMain(m *testing.M) {
	if len(os.Args) == 2 {
		mode := ReaderWorker
		switch os.Args[1] {
		case "worker-reader":
		case "worker-publisher":
			mode = PublisherWorker
		default:
			os.Exit(m.Run())
		}
		if err := runEscapeProbeWorker(mode); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runEscapeProbeWorker tries the exact host effects that the installed unit
// must deny before it participates in the ordinary worker protocol. TCP, file
// and host-IPC success are immediate failures; UDP is observed independently
// by the Endpoint-side positive control because a connected datagram socket
// can report success before a packet reaches a listener.
func runEscapeProbeWorker(mode WorkerMode) error {
	if err := verifyWorkerDescriptors(); err != nil {
		return err
	}
	if err := requireEscapeRefusals(); err != nil {
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

func requireEscapeRefusals() error {
	refuse := func(name string, attempt func() error) error {
		if err := attempt(); err == nil {
			return errors.New("escape probe reached host " + name)
		}
		return nil
	}
	if err := refuse("IPv4 TCP", func() error {
		connection, err := net.DialTimeout("tcp4", escapeProbeTCP4, 500*time.Millisecond)
		if connection != nil {
			_ = connection.Close()
		}
		return err
	}); err != nil {
		return err
	}
	if err := refuse("IPv6 TCP", func() error {
		connection, err := net.DialTimeout("tcp6", escapeProbeTCP6, 500*time.Millisecond)
		if connection != nil {
			_ = connection.Close()
		}
		return err
	}); err != nil {
		return err
	}
	if err := refuse("host file", func() error {
		file, err := os.Open(escapeProbeFile)
		if file != nil {
			_ = file.Close()
		}
		return err
	}); err != nil {
		return err
	}
	if err := refuse("host IPC", func() error {
		connection, err := net.DialTimeout("unix", escapeProbeIPC, 500*time.Millisecond)
		if connection != nil {
			_ = connection.Close()
		}
		return err
	}); err != nil {
		return err
	}
	if err := refuse("DNS", func() error {
		resolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "udp4", escapeProbeDNS)
		}}
		_, err := resolver.LookupHost(context.Background(), "ardents-worker-escape.invalid")
		return err
	}); err != nil {
		return err
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_UNSHARE, uintptr(syscall.CLONE_NEWNET), 0, 0); errno == 0 {
		return errors.New("escape probe created a network namespace")
	}
	if err := syscall.Setuid(0); err == nil {
		return errors.New("escape probe acquired root uid")
	}
	connection, err := net.Dial("udp4", escapeProbeUDP4)
	if err != nil {
		return nil
	}
	defer connection.Close()
	_, _ = connection.Write([]byte("ardents-text-worker-escape-probe"))
	return nil
}
