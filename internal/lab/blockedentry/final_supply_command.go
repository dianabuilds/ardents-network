package blockedentry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

func boundedReceiptCommand(name string, arguments ...string) ([]byte, error) {
	return boundedSupplyCommand(30*time.Second, finalReceiptOutputLimit, name, arguments...)
}

func boundedSupplyCommand(timeout time.Duration, limit int64, name string, arguments ...string) ([]byte, error) {
	return boundedSupplyCommandInput(timeout, limit, nil, name, arguments...)
}

func boundedSupplyCommandInput(timeout time.Duration, limit int64, input io.Reader,
	name string, arguments ...string,
) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, name, arguments...)
	command.Stdin = input
	prepareReceiptProcess(command)
	command.WaitDelay = 5 * time.Second
	command.Cancel = func() error { return terminateReceiptProcess(command) }
	stdout, stdoutErr := command.StdoutPipe()
	stderr, stderrErr := command.StderrPipe()
	if stdoutErr != nil || stderrErr != nil {
		return nil, errors.Join(stdoutErr, stderrErr)
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	type capture struct {
		value    []byte
		overflow bool
		err      error
	}
	read := func(input io.Reader) capture {
		var buffer bytes.Buffer
		written, err := io.CopyN(&buffer, input, limit+1)
		if err == io.EOF {
			err = nil
		}
		if written > limit {
			buffer.Truncate(int(limit))
		}
		_, drainErr := io.Copy(io.Discard, input)
		return capture{buffer.Bytes(), written > limit, errors.Join(err, drainErr)}
	}
	stdoutResult, stderrResult := make(chan capture, 1), make(chan capture, 1)
	go func() { stdoutResult <- read(stdout) }()
	go func() { stderrResult <- read(stderr) }()
	waitErr := command.Wait()
	out, diagnostic := <-stdoutResult, <-stderrResult
	if ctx.Err() != nil || out.overflow || diagnostic.overflow {
		return nil, errors.New("supply command exceeded its time or output bound")
	}
	if waitErr != nil || out.err != nil || diagnostic.err != nil {
		return nil, fmt.Errorf("supply command failed: %w: %s", errors.Join(waitErr, out.err, diagnostic.err),
			strings.TrimSpace(string(diagnostic.value)))
	}
	return out.value, nil
}
