//go:build installation_native

package generation

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

// This is an executable-file mechanism check only, never Release or startup
// authority. The original Owner remains open while the actual native exec joins.
func TestInstallationNativeCompletedExecutableDoesNotRetainWriter(t *testing.T) {
	directory, owned := nativeGeneration(t)
	program, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Fatal("invalid environment: native true executable required", err)
	}
	for _, name := range Names() {
		body := []byte("closed inventory bytes\n")
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			body = program
		}
		if _, err := owned.CreateFile(t.Context(), name); err != nil {
			t.Fatal(err)
		}
		if err := owned.Write(t.Context(), name, body); err != nil {
			t.Fatal(err)
		}
		// A read-only retained descriptor must reject even a same-byte write.
		if _, err := owned.files[name].file.WriteAt(body[:1], 0); err == nil {
			t.Fatal("completed leaf retained mutation capability")
		}
	}
	if err := owned.Seal(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ardents-linux-amd64", "ardents-text-linux-amd64"} {
		cmd := exec.CommandContext(t.Context(), filepath.Join(directory, name))
		cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
		runtime.LockOSThread()
		out, err := cmd.CombinedOutput()
		runtime.UnlockOSThread()
		if err != nil {
			t.Fatalf("original sealed executable could not exec: %v: %s", err, out)
		}
		writer, err := os.OpenFile(filepath.Join(directory, name), os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Independent kernel negative control: the same unchanged executable
		// cannot run while any descriptor keeps write access to its inode.
		cmd = exec.CommandContext(t.Context(), filepath.Join(directory, name))
		cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
		runtime.LockOSThread()
		out, err = cmd.CombinedOutput()
		runtime.UnlockOSThread()
		closeErr := writer.Close()
		if !errors.Is(err, syscall.ETXTBSY) || closeErr != nil {
			t.Fatalf("kernel writer control: %v, close %v: %s", err, closeErr, out)
		}
	}
	if err := owned.Observe(); err != nil {
		t.Fatal(err)
	}
	if err := owned.Close(); err != nil {
		t.Fatal(err)
	}
}
