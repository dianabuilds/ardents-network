//go:build installation_native

package completion

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestInstallationNativeCompletionRefusesForeignRootPeerGroup(t *testing.T) {
	if directory := os.Getenv("ARDENTS_COMPLETION_FOREIGN_PEER"); directory != "" {
		if os.Geteuid() != 0 || os.Getegid() != 65534 {
			t.Fatal("invalid actual peer credentials")
		}
		listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(directory, "start-completion.socket"), Net: "unix"})
		if err != nil {
			t.Fatal(err)
		}
		listener.SetUnlinkOnClose(false)
		if err := errors.Join(os.Chown(listener.Addr().String(), 0, 65534), os.Chmod(listener.Addr().String(), 0660), listener.SetDeadline(time.Now().Add(3*time.Second))); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stdout.Write([]byte("ready\n")); err != nil {
			t.Fatal(err)
		}
		peer, err := listener.AcceptUnix()
		if err != nil {
			t.Fatal(err)
		}
		if err := peer.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var first [1]byte
		if n, err := peer.Read(first[:]); n != 0 || !errors.Is(err, io.EOF) {
			t.Error("foreign peer received frame bytes", n, err)
		}
		if err := errors.Join(peer.Close(), listener.Close()); err != nil {
			t.Fatal(err)
		}
		return
	}
	directory, listener, frame := completionFixture(t)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, "start-completion.socket")); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestInstallationNativeCompletionRefusesForeignRootPeerGroup$", "-test.count=1")
	command.Env = []string{"ARDENTS_COMPLETION_FOREIGN_PEER=" + directory}
	command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 0, Gid: 65534}}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var diagnostic bytes.Buffer
	command.Stderr = &diagnostic
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	var ready [6]byte
	_, readErr := io.ReadFull(output, ready[:])
	if readErr != nil || string(ready[:]) != "ready\n" {
		cancel()
		t.Fatal("foreign peer did not become ready", readErr, command.Wait(), diagnostic.String())
	}
	info, err := os.Lstat(directory)
	if err != nil {
		cancel()
		t.Fatal(err, command.Wait())
	}
	e, connectErr := Connect(ctx, directory, info, 65534, frame)
	if e != nil {
		e.Close()
	}
	if !errors.Is(connectErr, ErrBinding) || e != nil {
		t.Error("foreign root peer group accepted", connectErr)
	}
	if _, err := io.Copy(io.Discard, output); err != nil {
		t.Error(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatal("original peer process join", err, diagnostic.String())
	}
}

func completionFixture(t *testing.T) (string, *net.UnixListener, [160]byte) {
	t.Helper()
	parent := os.Getenv("ARDENTS_INSTALLATION_NATIVE_ROOT")
	if os.Geteuid() != 0 || os.Getegid() != 0 || !filepath.IsAbs(parent) {
		t.Fatal("invalid environment: root driver and trusted native parent required")
	}
	for current := parent; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
			t.Fatal("invalid native parent", err)
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || native.Uid != 0 {
			t.Fatal("foreign native parent")
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	directory, err := os.MkdirTemp(parent, "installation-completion-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	if err := errors.Join(os.Chown(directory, 0, 65534), os.Chmod(directory, 0750)); err != nil {
		t.Fatal(err)
	}
	// These metadata fixtures are deliberately not canonical Installation
	// intents. Wire mechanism success grants no archival or startup authority.
	for _, name := range []string{"start-guard.json", "start-socket.json"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("mechanism metadata only\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(directory, "start-completion.socket"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	})
	if err := errors.Join(os.Chown(listener.Addr().String(), 0, 65534), os.Chmod(listener.Addr().String(), 0660), listener.SetDeadline(time.Now().Add(5*time.Second))); err != nil {
		t.Fatal(err)
	}
	frame := [160]byte([]byte(strings.Repeat("1", 32) + strings.Repeat("a", 64) + strings.Repeat("b", 64)))
	return directory, listener, frame
}

func connectFixture(t *testing.T, ctx context.Context, directory string, frame [160]byte) *Exchange {
	t.Helper()
	info, err := os.Lstat(directory)
	if err != nil {
		t.Fatal(err)
	}
	e, err := Connect(ctx, directory, info, 65534, frame)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestInstallationNativeCompletionExactReplyAndPhysicalClose(t *testing.T) {
	for _, mode := range []string{"exact", "changed", "short"} {
		t.Run(mode, func(t *testing.T) {
			directory, listener, frame := completionFixture(t)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			e := connectFixture(t, ctx, directory, frame)
			root, connection := e.root, e.connection
			done := make(chan error, 1)
			go func() {
				peer, err := listener.AcceptUnix()
				if err != nil {
					done <- err
					return
				}
				var request [160]byte
				_, err = io.ReadFull(peer, request[:])
				if err == nil && request != frame {
					err = ErrBinding
				}
				reply := frame
				if mode == "changed" {
					reply[80] = 'c'
				}
				body := reply[:]
				if mode == "short" {
					body = body[:159]
				}
				if err == nil {
					_, err = io.Copy(peer, bytes.NewReader(body))
				}
				done <- errors.Join(err, peer.Close())
			}()
			err := e.Wait()
			if mode == "exact" && err != nil {
				t.Error("exact mechanism reply refused", err)
			}
			if mode == "changed" && !errors.Is(err, ErrBinding) {
				t.Error("changed reply accepted", err)
			}
			if mode == "short" && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Error("short reply accepted", err)
			}
			if err := <-done; err != nil {
				t.Error("original peer join", err)
			}
			if mode == "exact" {
				if err := e.Close(); err != nil {
					t.Error(err)
				}
			} else {
				if err := e.Close(); err == nil {
					t.Error("close lost first exchange failure")
				}
			}
			if _, err := root.Stat("."); err == nil {
				t.Error("directory descriptor not physically closed")
			}
			raw, closedErr := connection.SyscallConn()
			if closedErr == nil {
				closedErr = raw.Control(func(uintptr) {})
			}
			if closedErr == nil {
				t.Error("connection not physically closed")
			}
			if err := e.Wait(); !errors.Is(err, ErrInput) {
				t.Error("exchange replay accepted", err)
			}
		})
	}
}

func TestInstallationNativeCompletionCancellationJoinsBlockedIO(t *testing.T) {
	directory, listener, frame := completionFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	e := connectFixture(t, ctx, directory, frame)
	peer, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- e.Wait() }()
	var request [160]byte
	if _, err := io.ReadFull(peer, request[:]); err != nil {
		t.Fatal(err)
	}
	if request != frame {
		t.Fatal("outgoing frame differs")
	}
	cancel()
	if err := <-waited; !errors.Is(err, context.Canceled) {
		t.Fatal("blocked exchange lost original cancellation", err)
	}
	var one [1]byte
	if _, err := peer.Read(one[:]); !errors.Is(err, io.EOF) {
		t.Fatal("cancelled client retained physical socket", err)
	}
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); !errors.Is(err, context.Canceled) {
		t.Fatal("close lost first cancellation", err)
	}
}

func TestInstallationNativeCompletionRefusesOriginalMetadataSubstitutionBeforeOutput(t *testing.T) {
	for _, name := range []string{"start-guard.json", "start-socket.json", "start-completion.socket"} {
		t.Run(name, func(t *testing.T) {
			directory, listener, frame := completionFixture(t)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			e := connectFixture(t, ctx, directory, frame)
			peer, err := listener.AcceptUnix()
			if err != nil {
				t.Fatal(err)
			}
			filename := filepath.Join(directory, name)
			if err := os.Rename(filename, filename+".original"); err != nil {
				t.Fatal(err)
			}
			if name == "start-completion.socket" {
				other, err := net.ListenUnix("unix", &net.UnixAddr{Name: filename, Net: "unix"})
				if err != nil {
					t.Fatal(err)
				}
				other.SetUnlinkOnClose(false)
				if err := errors.Join(os.Chown(filename, 0, 65534), os.Chmod(filename, 0660), other.Close()); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(filename, []byte("mechanism metadata only\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := e.Wait(); !errors.Is(err, ErrBinding) {
				t.Error("substitution accepted", err)
			}
			if err := e.Close(); !errors.Is(err, ErrBinding) {
				t.Error("close lost substitution", err)
			}
			var first [1]byte
			if n, err := peer.Read(first[:]); n != 0 || !errors.Is(err, io.EOF) {
				t.Error("frame output preceded original metadata check", n, err)
			}
			if err := peer.Close(); err != nil {
				t.Error(err)
			}
		})
	}
}
