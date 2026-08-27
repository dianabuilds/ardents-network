//go:build ignore

// R-102 is a disposable two-process Endpoint liveness-lock experiment.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

func main() {
	root := flag.String("root", "", "synthetic state root")
	holdReady := flag.String("hold-ready", "", "write marker after ownership and bind, then wait")
	expectBusy := flag.Bool("expect-busy", false, "succeed only when another owner holds the lock")
	recoverSocket := flag.Bool("recover-socket", false, "recover one stale runtime socket after ownership")
	flag.Parse()
	if *root == "" {
		fail("missing_root")
	}
	stateRoot := filepath.Join(*root, "endpoint-state")
	liveRoot := filepath.Join(stateRoot, "live")
	runtimeRoot := filepath.Join(stateRoot, "runtime")
	if err := ensureDirectory(*root); err != nil {
		fail("unexpected_root: " + err.Error())
	}
	if err := ensureDirectory(stateRoot); err != nil {
		fail("unexpected_state_root: " + err.Error())
	}
	if err := ensureDirectory(liveRoot); err != nil {
		fail("unexpected_live_root: " + err.Error())
	}
	if err := ensureDirectory(runtimeRoot); err != nil {
		fail("unexpected_runtime_root: " + err.Error())
	}
	if err := ensureDirectory(filepath.Join(stateRoot, "vault")); err != nil {
		fail("unexpected_vault_root: " + err.Error())
	}
	release, busy, err := acquireOwnerLock(filepath.Join(liveRoot, "owner.lock"))
	if err != nil {
		fail("acquire_lock: " + err.Error())
	}
	if busy {
		if *expectBusy {
			fmt.Println("lock=busy")
			return
		}
		fail("lock_busy")
	}
	if *expectBusy {
		fail("lock_unexpectedly_acquired")
	}
	defer func() {
		if err := release(); err != nil {
			fail("release_lock: " + err.Error())
		}
	}()

	socketPath := filepath.Join(runtimeRoot, "attachment.sock")
	if *recoverSocket {
		recover(socketPath)
		fmt.Println("lock=acquired")
		fmt.Println("stale_socket_type=socket")
		fmt.Println("stale_socket_recovered=true")
		return
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fail("listen_socket: " + err.Error())
	}
	defer listener.Close()
	if err := secureAttachment(socketPath); err != nil {
		fail("secure_socket: " + err.Error())
	}
	if *holdReady == "" {
		fmt.Println("lock=acquired")
		return
	}
	if err := os.WriteFile(*holdReady, []byte("ready\n"), 0o600); err != nil {
		fail("write_ready: " + err.Error())
	}
	for {
		time.Sleep(time.Hour)
	}
}

func ensureDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(path, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("expected an owned directory, not a link or another entry type")
	}
	return secureOwnedDirectory(path, info)
}

func recover(socketPath string) {
	info, err := os.Lstat(socketPath)
	if err != nil {
		fail("inspect_stale_socket: " + err.Error())
	}
	if info.Mode()&os.ModeSocket == 0 {
		fail("unexpected_runtime_entry_type")
	}
	if err := os.Remove(socketPath); err != nil {
		fail("remove_stale_socket: " + err.Error())
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fail("rebind_socket: " + err.Error())
	}
	if err := secureAttachment(socketPath); err != nil {
		_ = listener.Close()
		fail("secure_rebound_socket: " + err.Error())
	}
	if err := listener.Close(); err != nil {
		fail("close_rebound_socket: " + err.Error())
	}
}

func fail(reason string) {
	fmt.Fprintln(os.Stderr, "failure="+reason)
	os.Exit(1)
}
