//go:build linux

package main

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"
)

// openDocumentCommandIO gives the trusted UI separate pollable file descriptions.
// Dup+SetNonblock would change the inherited open-file description shared with
// the invoking shell. Reopening the exact proc descriptor avoids that mutation.
// This is never called by either fixed worker entrypoint.
func openDocumentCommandIO() (*os.File, *os.File, error) {
	input, err := openDocumentPresentationDescriptor("/proc/self/fd/0", os.Stdin, os.O_RDONLY)
	if err != nil {
		return nil, nil, err
	}
	output, err := openDocumentOutput()
	if err != nil {
		return nil, nil, errors.Join(err, input.Close())
	}
	// Refuse descriptors on which Go cannot interrupt a pending UI operation.
	// In particular, this command offers terminal/pipe presentation, not export
	// to an arbitrary regular output file.
	if err := input.SetReadDeadline(time.Time{}); err != nil {
		return nil, nil, errors.Join(err, input.Close(), output.Close())
	}
	return input, output, nil
}

func openDocumentPresentationDescriptor(path string, inherited *os.File, mode int) (*os.File, error) {
	before, err := inherited.Stat()
	if err != nil {
		return nil, err
	}
	opened, err := os.OpenFile(path, mode|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	after, err := opened.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, errors.Join(errors.New("document UI descriptor changed"), err, opened.Close())
	}
	return opened, nil
}

func runPublishedLink(ctx context.Context, socket string) error {
	output, err := openDocumentOutput()
	if err != nil {
		return err
	}
	return showPublishedLink(ctx, socket, output)
}

func openDocumentOutput() (*os.File, error) {
	output, err := openDocumentPresentationDescriptor("/proc/self/fd/1", os.Stdout, os.O_WRONLY)
	if err != nil {
		return nil, err
	}
	if err := output.SetWriteDeadline(time.Time{}); err != nil {
		return nil, errors.Join(err, output.Close())
	}
	return output, nil
}

// openPrivateLinkFile refuses a substituted final symlink and opens without
// blocking on a non-regular file. The caller checks the opened file's mode.
func openPrivateLinkFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}

// NewFile notices O_NONBLOCK and attaches inherited pipes to Go's poller.
// Closing the owned duplicate then interrupts Read/Write instead of leaving a
// thread trapped in a blocking syscall on the original inherited descriptor.
func admissionConsoleFile(file *os.File) (*os.File, error) {
	fd, err := syscall.Dup(int(file.Fd()))
	if err != nil {
		return nil, err
	}
	syscall.CloseOnExec(fd)
	if err = syscall.SetNonblock(fd, true); err != nil {
		return nil, errors.Join(err, syscall.Close(fd))
	}
	owned := os.NewFile(uintptr(fd), "admission-console")
	if owned == nil {
		return nil, errors.Join(errors.New("console unavailable"), syscall.Close(fd))
	}
	return owned, nil
}
