//go:build ignore

// R-101 is a disposable ordinary-user state and local-attachment experiment.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

func main() {
	root := flag.String("root", "", "empty experiment root")
	holdReady := flag.String("hold-ready", "", "write readiness marker then wait for process termination")
	flag.Parse()
	if *root == "" {
		fail("missing_root")
	}
	if _, err := os.Lstat(*root); !errors.Is(err, os.ErrNotExist) {
		fail("root_must_not_exist")
	}

	stateRoot := filepath.Join(*root, "endpoint-state")
	artifactRoot := filepath.Join(*root, "artifact")
	classes := []string{
		"vault", "configuration-grants", "floors", "cache", "live",
		"diagnostics", "runtime",
	}
	for _, class := range classes {
		if err := os.MkdirAll(filepath.Join(stateRoot, class), 0o700); err != nil {
			fail("state_class_" + class + ": " + err.Error())
		}
	}
	if err := os.MkdirAll(artifactRoot, 0o700); err != nil {
		fail("artifact_root: " + err.Error())
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "ardents-fixture"), []byte("replaceable"), 0o600); err != nil {
		fail("artifact_fixture: " + err.Error())
	}

	ownerPath := filepath.Join(stateRoot, "live", "owner.lock")
	owner, err := os.OpenFile(ownerPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		fail("first_owner: " + err.Error())
	}
	defer owner.Close()
	duplicateOwnerRejected := false
	if other, err := os.OpenFile(ownerPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); err != nil {
		duplicateOwnerRejected = errors.Is(err, os.ErrExist)
	} else {
		other.Close()
	}
	if !duplicateOwnerRejected {
		fail("duplicate_owner_accepted")
	}

	socketPath := filepath.Join(stateRoot, "runtime", "attachment.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fail("listen_unix: " + err.Error())
	}
	defer listener.Close()
	secondListenerRejected := false
	if duplicate, err := net.Listen("unix", socketPath); err != nil {
		secondListenerRejected = true
	} else {
		duplicate.Close()
	}
	if !secondListenerRejected {
		fail("duplicate_listener_accepted")
	}
	if *holdReady != "" {
		if err := os.WriteFile(*holdReady, []byte("ready\n"), 0o600); err != nil {
			fail("write_hold_ready: " + err.Error())
		}
		for {
			time.Sleep(time.Hour)
		}
	}

	served := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			served <- err
			return
		}
		defer conn.Close()
		request := make([]byte, 5)
		if _, err := io.ReadFull(conn, request); err != nil {
			served <- err
			return
		}
		if string(request) != "probe" {
			served <- fmt.Errorf("unexpected request %q", request)
			return
		}
		_, err = conn.Write([]byte("ready"))
		served <- err
	}()
	client, err := net.Dial("unix", socketPath)
	if err != nil {
		fail("dial_unix: " + err.Error())
	}
	if _, err := client.Write([]byte("probe")); err != nil {
		client.Close()
		fail("write_unix: " + err.Error())
	}
	response := make([]byte, 5)
	if _, err := io.ReadFull(client, response); err != nil {
		client.Close()
		fail("read_unix: " + err.Error())
	}
	client.Close()
	if err := <-served; err != nil {
		fail("serve_unix: " + err.Error())
	}
	if string(response) != "ready" {
		fail("unexpected_response")
	}

	if err := listener.Close(); err != nil {
		fail("close_listener: " + err.Error())
	}
	staleAfterClose := false
	if _, err := os.Lstat(socketPath); err == nil {
		staleAfterClose = true
		if err := os.Remove(socketPath); err != nil {
			fail("remove_stale_socket: " + err.Error())
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		fail("inspect_socket_after_close: " + err.Error())
	}
	if recovered, err := net.Listen("unix", socketPath); err != nil {
		fail("recover_listener: " + err.Error())
	} else if err := recovered.Close(); err != nil {
		fail("close_recovered_listener: " + err.Error())
	}

	if err := os.RemoveAll(artifactRoot); err != nil {
		fail("remove_artifact: " + err.Error())
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "vault")); err != nil {
		fail("state_lost_with_artifact: " + err.Error())
	}

	fmt.Println("state_classes=7")
	fmt.Println("duplicate_owner_rejected=true")
	fmt.Println("duplicate_listener_rejected=true")
	fmt.Println("local_round_trip=ready")
	fmt.Printf("stale_socket_after_close=%t\n", staleAfterClose)
	fmt.Println("listener_recovery=true")
	fmt.Println("artifact_state_separation=true")
}

func fail(reason string) {
	fmt.Fprintln(os.Stderr, "failure="+reason)
	os.Exit(1)
}
