//go:build linux && endpointstream_native

package targetlink_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// This selected Linux profile uses two actual unprivileged numeric UIDs.
// Root and the ability to change ownership and credentials are prerequisites.
func TestPrivateLinkOwnerAndForeignUserBeforeDial(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("the native private-input profile requires root")
	}
	const owner, foreign = 17351, 17352
	directory, err := os.MkdirTemp("/tmp", "ardents-link-owner-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(directory)
	if err := os.Chmod(directory, 0755); err != nil {
		t.Fatal(err)
	}
	// The ordinary build helper's temporary directory is private to root.
	// Copy its exact executable to this traversable, test-owned directory.
	source, err := os.Open(buildArdentsNext(t))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(directory, "ardents-next")
	destination, err := os.OpenFile(binary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(destination, source)
	if err := errors.Join(copyErr, source.Close(), destination.Close()); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "private-link")
	if err := os.WriteFile(link, []byte("fixture-link\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(link, owner, owner); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name string
		uid  uint32
		want string
	}{
		{"owner reaches dial", owner, "local stream admission failed"},
		{"foreign user refuses before dial", foreign, "private Target Link file is unavailable"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary, "stream", "connect", filepath.Join(directory, "missing.sock"), "--link-file", link)
			command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: scenario.uid, Gid: scenario.uid}}
			input, inputWriter, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			defer inputWriter.Close()
			output, outputWriter, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			defer outputWriter.Close()
			// The CLI reopens its descriptors through procfs. They must be
			// owned by the actual child UID, as in a user's own shell.
			if err := errors.Join(input.Chown(int(scenario.uid), int(scenario.uid)), outputWriter.Chown(int(scenario.uid), int(scenario.uid)), inputWriter.Close()); err != nil {
				t.Fatal(err)
			}
			var diagnostic bytes.Buffer
			command.Stdin, command.Stdout, command.Stderr = input, outputWriter, &diagnostic
			err = command.Run()
			closeErr := outputWriter.Close()
			stdout, readErr := io.ReadAll(output)
			want := "Application stream failed: " + scenario.want + "\n" + `{"operation":"application.stream.connect","phase":"command","outcome":"failed"}` + "\n"
			if ctx.Err() != nil || err == nil || closeErr != nil || readErr != nil || len(stdout) != 0 || diagnostic.String() != want {
				t.Fatalf("error=%v, context=%v, stdout=%q, stderr=%q, close=%v, read=%v", err, ctx.Err(), stdout, &diagnostic, closeErr, readErr)
			}
		})
	}
}
