//go:build linux && text_worker_installed

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	pty "github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/unix"
)

const installedServiceCustodyPassword = "closed process custody fixture password"

type installedServiceCustodyInput struct {
	prompt, value string
	secret        bool
}

type installedServiceCustodyCapture struct {
	mu     sync.Mutex
	output bytes.Buffer
	cancel context.CancelFunc
}

func (capture *installedServiceCustodyCapture) Write(value []byte) (int, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if len(value) > (128<<10)-capture.output.Len() {
		capture.cancel()
		return 0, io.ErrShortBuffer
	}
	return capture.output.Write(value)
}
func (capture *installedServiceCustodyCapture) bytes() []byte {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]byte(nil), capture.output.Bytes()...)
}

// Passwords enter a genuine terminal only after the command disables echo.
// Cancellation and normal completion both join the child and output reader.
func installedServiceCustodyCommand(t *testing.T, binary string, inputs []installedServiceCustodyInput, arguments ...string) (output []byte, resultErr error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	terminal, err := pty.New()
	if err != nil {
		return nil, err
	}
	unixTerminal, ok := terminal.(pty.UnixPty)
	if !ok {
		return nil, errors.Join(errors.New("Custody terminal is not a Unix terminal"), terminal.Close())
	}
	command := terminal.CommandContext(ctx, binary, arguments...)
	if err := command.Start(); err != nil {
		return nil, errors.Join(err, terminal.Close())
	}
	capture := &installedServiceCustodyCapture{cancel: cancel}
	readDone, waitDone := make(chan error, 1), make(chan error, 1)
	go func() { _, err := io.Copy(capture, terminal); readDone <- err }()
	go func() { waitDone <- command.Wait() }()
	waited := false
	defer func() {
		cancel()
		if !waited {
			select {
			case err := <-waitDone:
				resultErr = errors.Join(resultErr, err)
			case <-time.After(5 * time.Second):
				resultErr = errors.Join(resultErr, errors.New("Custody process did not join"))
			}
		}
		// Closing the parent's slave lets the reader consume the child's final
		// bytes before Linux reports EIO. Closing master first loses that tail.
		resultErr = errors.Join(resultErr, unixTerminal.Slave().Close())
		masterClosed := false
		select {
		case err := <-readDone:
			if err != nil && !errors.Is(err, syscall.EIO) && !errors.Is(err, os.ErrClosed) && !errors.Is(err, io.EOF) {
				resultErr = errors.Join(resultErr, err)
			}
		case <-time.After(5 * time.Second):
			resultErr = errors.Join(resultErr, errors.New("Custody terminal reader did not drain"), unixTerminal.Master().Close())
			masterClosed = true
			select {
			case <-readDone:
			case <-time.After(5 * time.Second):
				resultErr = errors.Join(resultErr, errors.New("Custody terminal reader did not join after close"))
			}
		}
		if !masterClosed {
			resultErr = errors.Join(resultErr, unixTerminal.Master().Close())
		}
		output = capture.bytes()
		if bytes.Contains(output, []byte(installedServiceCustodyPassword)) {
			output = nil
			resultErr = errors.Join(resultErr, errors.New("Custody echoed a test password"))
		}
	}()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for _, input := range inputs {
		for {
			prompt := bytes.Contains(capture.bytes(), []byte(input.prompt))
			if prompt && input.secret {

				settings, err := unix.IoctlGetTermios(int(unixTerminal.Slave().Fd()), unix.TCGETS)
				if err != nil {
					return nil, err
				}
				prompt = settings.Lflag&unix.ECHO == 0
			}
			if prompt {
				break
			}
			select {
			case err := <-waitDone:
				waited = true
				return nil, fmt.Errorf("Custody exited before prompt %q: %v", input.prompt, err)
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-ticker.C:
			}
		}
		if _, err := io.WriteString(terminal, input.value+"\n"); err != nil {
			return nil, err
		}
	}
	select {
	case err := <-waitDone:
		waited = true
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
