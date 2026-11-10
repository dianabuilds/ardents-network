package forwarding

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// These are native file/process mechanism tests, not Completed or Startup
// admission. A child executes its own original inode; no manager, binding,
// Release, State or successful domain authority is supplied.
func TestCompletedExecutableViewPhysicalLifetime(t *testing.T) {
	if mode := os.Getenv("ARDENTS_TEST_EXECUTABLE_VIEW"); mode != "" {
		completedExecutableViewChild(t, mode)
		return
	}
	for _, mode := range []string{"cancel", "descriptor-loss", "changed-reference"} {
		t.Run(mode, func(t *testing.T) {
			program := copyExecutableViewTestProgram(t)
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, program, "-test.run=^TestCompletedExecutableViewPhysicalLifetime$", "-test.timeout=20s")
			command.Env = append(os.Environ(), "ARDENTS_TEST_EXECUTABLE_VIEW="+mode)
			stdout, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdin, err := command.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr strings.Builder
			command.Stderr = &stderr
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			waited := false
			t.Cleanup(func() {
				if !waited {
					_ = command.Process.Kill()
					_ = command.Wait()
				}
			})
			reader := bufio.NewReader(stdout)
			line, err := reader.ReadString('\n')
			if err != nil || line != "mapped-byte-borrow-held\n" {
				t.Fatalf("actual executing view not held: %q %v", line, err)
			}
			// The copy is owner-writable. The actual kernel executing-inode
			// exclusion, rather than permissions or an injected verdict, must
			// prevent a writer even when this parent has root privileges.
			writer, err := os.OpenFile(program, os.O_WRONLY, 0)
			if writer != nil {
				_ = writer.Close()
			}
			if !errors.Is(err, syscall.ETXTBSY) {
				t.Fatal("live original executable allowed a writer", err)
			}
			if _, err := io.WriteString(stdin, "continue\n"); err != nil {
				t.Fatal(err)
			}
			_ = stdin.Close()
			output, readErr := io.ReadAll(reader)
			waitErr := command.Wait()
			waited = true
			if readErr != nil || waitErr != nil || !strings.Contains(string(output), "original-view-and-descriptor-joined\n") {
				t.Fatalf("original physical lifetime failed: %v %v\n%s\n%s", readErr, waitErr, output, stderr.String())
			}
		})
	}
}

func copyExecutableViewTestProgram(t *testing.T) string {
	t.Helper()
	self, err := os.Open("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	defer self.Close()
	path := filepath.Join(t.TempDir(), "program")
	copy, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := io.Copy(copy, self)
	closeErr := copy.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal(writeErr, closeErr)
	}
	return path
}

func completedExecutableViewChild(t *testing.T, mode string) {
	t.Helper()
	if mode != "cancel" && mode != "descriptor-loss" && mode != "changed-reference" {
		t.Fatal("unknown physical control")
	}
	file, err := os.Open("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(file)
	if err != nil || len(body) <= 64<<10 {
		t.Fatal("actual multi-chunk executable required", err)
	}
	original := &initialFile{file: file, info: info, body: body}
	view, err := openCompletedExecutableView(original)
	if err != nil || view == nil {
		t.Fatal("actual own executable not mapped", err)
	}
	address := reflect.ValueOf(view.body).Pointer()
	inode := info.Sys().(*syscall.Stat_t).Ino
	if !completedExecutableRegion(t, address, inode) {
		t.Fatal("view is not an actual read-only file-backed mapping")
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	owner := &completedFiles{ctx: ctx, fileOrder: []*initialFile{original}, executable: view}
	t.Cleanup(func() { _ = owner.close() })
	entered, release, denied := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var first, released sync.Once
	t.Cleanup(func() { released.Do(func() { close(release) }) })
	owner.observe = func(phase string, _ *os.File) {
		switch phase {
		case "executable-chunk":
			first.Do(func() { close(entered); <-release })
		case "denied":
			close(denied)
		}
	}
	read := make(chan error, 1)
	go func() {
		if err := owner.begin(); err != nil {
			read <- err
			return
		}
		owner.ioMu.Lock()
		var scratch [64 << 10]byte
		err := owner.checkFileBytes(original, scratch[:])
		clear(scratch[:])
		owner.record(err)
		owner.ioMu.Unlock()
		owner.uses.Done()
		read <- err
	}()
	<-entered
	if _, err := io.WriteString(os.Stdout, "mapped-byte-borrow-held\n"); err != nil {
		t.Fatal(err)
	}
	if line, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil || line != "continue\n" {
		t.Fatal("original writer-denial control did not join", err)
	}
	var closed error
	switch mode {
	case "cancel":
		cancel()
		completion := make(chan error, 1)
		go func() { completion <- owner.close() }()
		<-denied
		if len(view.body) != len(body) {
			t.Fatal("view released before its original byte reader joined")
		}
		if _, err := original.file.Stat(); err != nil {
			t.Fatal("descriptor released before its original reader joined", err)
		}
		select {
		case err := <-completion:
			t.Fatal("Close returned before physical byte borrow joined", err)
		default:
		}
		released.Do(func() { close(release) })
		if err := <-read; !errors.Is(err, context.Canceled) || errors.Is(err, ErrBinding) {
			t.Fatal("original cancellation lost", err)
		}
		closed = <-completion
		if !errors.Is(closed, context.Canceled) || errors.Is(closed, os.ErrClosed) {
			t.Fatal("joined cancellation changed", closed)
		}
	case "descriptor-loss":
		if err := original.file.Close(); err != nil {
			t.Fatal(err)
		}
		released.Do(func() { close(release) })
		if err := <-read; !errors.Is(err, ErrBinding) || !errors.Is(err, os.ErrClosed) {
			t.Fatal("mapped comparison omitted independent original EOF", err)
		}
		closed = owner.close()
		if !errors.Is(closed, os.ErrClosed) {
			t.Fatal("original descriptor loss erased during Close", closed)
		}
	case "changed-reference":
		// The first real chunk is held. A later byte of the independently
		// retained reference changes before the mapped reader continues.
		body[len(body)-1] ^= 1
		released.Do(func() { close(release) })
		if err := <-read; !errors.Is(err, ErrBinding) {
			t.Fatal("mapped path reused an earlier comparison verdict", err)
		}
		closed = owner.close()
	}
	if len(view.body) != 0 || closed != owner.close() {
		t.Fatal("original view not unmapped once with a retained result", closed)
	}
	if completedExecutableRegion(t, address, inode) {
		t.Fatal("original file mapping survived physical Close")
	}
	if _, err := original.file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("original descriptor survived joined Close", err)
	}
	if err := owner.begin(); err == nil {
		t.Fatal("late caller borrowed the closed original view")
	}
	if _, err := io.WriteString(os.Stdout, "original-view-and-descriptor-joined\n"); err != nil {
		t.Fatal(err)
	}
}

// Kernel mapping identity is an independent birth/retirement oracle. Pointer
// values are compared numerically only; no address is dereferenced or exposed.
func completedExecutableRegion(t *testing.T, address uintptr, inode uint64) bool {
	t.Helper()
	raw, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[1] != "r--s" || fields[2] != "00000000" {
			continue
		}
		rangeEnds := strings.Split(fields[0], "-")
		if len(rangeEnds) != 2 {
			t.Fatal("invalid native mapping range")
		}
		start, startErr := strconv.ParseUint(rangeEnds[0], 16, 64)
		end, endErr := strconv.ParseUint(rangeEnds[1], 16, 64)
		actual, inodeErr := strconv.ParseUint(fields[4], 10, 64)
		if startErr != nil || endErr != nil || inodeErr != nil {
			t.Fatal("invalid native file mapping", startErr, endErr, inodeErr)
		}
		if uint64(address) >= start && uint64(address) < end && actual == inode {
			return true
		}
	}
	return false
}

func TestCompletedExecutableViewDoesNotMapAnotherInode(t *testing.T) {
	path := copyExecutableViewTestProgram(t)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	view, err := openCompletedExecutableView(&initialFile{file: file, info: info, body: body})
	if view != nil {
		_ = view.close()
		t.Fatal("byte-identical foreign inode received an executable view")
	}
	if err != nil {
		t.Fatal("ordinary descriptor observation became unavailable", err)
	}
	// The foreign image remains mutable; it must keep actual descriptor reads.
	writer, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WriteAt([]byte{body[len(body)-1] ^ 1}, int64(len(body)-1)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := (&completedFiles{ctx: t.Context()}).checkFileBytes(&initialFile{file: file, body: body}, make([]byte, 64<<10)); !errors.Is(err, ErrBinding) {
		t.Fatal("foreign file stopped checking its actual bytes", err)
	}
}
