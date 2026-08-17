package blockedverify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os/exec"
	"time"
)

func boundedSupplyGitOutput(name string, arguments ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, arguments...)
	prepareSupplyProcess(command)
	command.Cancel = func() error { return terminateSupplyProcess(command) }
	command.WaitDelay = 5 * time.Second
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
		var output bytes.Buffer
		written, err := io.CopyN(&output, input, (1<<20)+1)
		if errors.Is(err, io.EOF) {
			err = nil
		}
		if written > 1<<20 {
			output.Truncate(1 << 20)
		}
		_, drainErr := io.Copy(io.Discard, input)
		return capture{output.Bytes(), written > 1<<20, errors.Join(err, drainErr)}
	}
	outs, diagnostics := make(chan capture, 1), make(chan capture, 1)
	go func() { outs <- read(stdout) }()
	go func() { diagnostics <- read(stderr) }()
	waitErr := command.Wait()
	out, diagnostic := <-outs, <-diagnostics
	if ctx.Err() != nil || out.overflow || diagnostic.overflow || waitErr != nil || out.err != nil || diagnostic.err != nil {
		return nil, errors.New("bounded final supply command failed")
	}
	return out.value, nil
}

func finalRepositoryArchiveHash(root, commit string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "-C", root, "archive", "--format=tar", commit)
	prepareSupplyProcess(command)
	command.Cancel = func() error { return terminateSupplyProcess(command) }
	command.WaitDelay = 5 * time.Second
	pipe, err := command.StdoutPipe()
	if err != nil {
		return "", err
	}
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		return "", err
	}
	digest := sha256.New()
	written, copyErr := io.Copy(digest, io.LimitReader(pipe, (1<<31)+1))
	if written > 1<<31 || copyErr != nil {
		_ = terminateSupplyProcess(command)
	}
	waitErr := command.Wait()
	if ctx.Err() != nil || written == 0 || written > 1<<31 || copyErr != nil || waitErr != nil {
		return "", errors.Join(copyErr, waitErr, errors.New("repository archive is unavailable or oversized"))
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
