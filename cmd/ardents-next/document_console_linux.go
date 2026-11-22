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
