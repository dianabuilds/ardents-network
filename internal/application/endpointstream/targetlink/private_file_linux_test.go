//go:build linux

package targetlink_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStreamConnectRefusesUnsafePrivateLinkBeforeDial(t *testing.T) {
	binary := buildArdentsNext(t)
	dir := t.TempDir()
	socket := filepath.Join(dir, "missing.sock")
	valid := filepath.Join(dir, "private-link")
	if err := os.WriteFile(valid, []byte("fixture-link\n"), 0600); err != nil {
		t.Fatal(err)
	}
	public := filepath.Join(dir, "public-link")
	if err := os.WriteFile(public, []byte("fixture-link\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(public, 0644); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "symlink")
	if err := os.Symlink(valid, symlink); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name, path, want string
	}{
		{"private", valid, "local stream admission failed"},
		{"group-readable", public, "private Target Link file is unavailable"},
		{"symlink", symlink, "private Target Link file is unavailable"},
		{"fifo", fifo, "private Target Link file is unavailable"},
		{"missing", filepath.Join(dir, "missing-link"), "private Target Link file is unavailable"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary, "stream", "connect", socket, "--link-file", scenario.path)
			command.Stdin = strings.NewReader("")
			var output, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &output, &diagnostic
			err := command.Run()
			if ctx.Err() != nil || err == nil || output.Len() != 0 ||
				diagnostic.String() != "Application stream failed: "+scenario.want+"\n"+`{"operation":"application.stream.connect","phase":"command","outcome":"failed"}`+"\n" ||
				strings.Contains(diagnostic.String(), scenario.path) {
				t.Fatalf("error=%v, context=%v, stdout=%q, stderr=%q", err, ctx.Err(), &output, &diagnostic)
			}
		})
	}
}
