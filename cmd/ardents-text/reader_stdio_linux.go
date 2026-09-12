//go:build linux

package main

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// openTextCommandIO gives the trusted UI separate pollable file descriptions.
// Dup+SetNonblock would change the inherited open-file description shared with
// the invoking shell. Reopening the exact proc descriptor avoids that mutation.
// This is never called by either fixed worker entrypoint.
func openTextCommandIO() (*os.File, *os.File, error) {
	input, err := openTextPresentationDescriptor("/proc/self/fd/0", os.Stdin, os.O_RDONLY)
	if err != nil {
		return nil, nil, err
	}
	output, err := openTextPresentationDescriptor("/proc/self/fd/1", os.Stdout, os.O_WRONLY)
	if err != nil {
		return nil, nil, errors.Join(err, input.Close())
	}
	// Refuse descriptors on which Go cannot interrupt a pending UI operation.
	// In particular, this command offers terminal/pipe presentation, not export
	// to an arbitrary regular output file.
	if err := errors.Join(input.SetReadDeadline(time.Time{}), output.SetWriteDeadline(time.Time{})); err != nil {
		return nil, nil, errors.Join(err, input.Close(), output.Close())
	}
	return input, output, nil
}

func openTextPresentationDescriptor(path string, inherited *os.File, mode int) (*os.File, error) {
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
		return nil, errors.Join(errors.New("text UI descriptor changed"), err, opened.Close())
	}
	return opened, nil
}
