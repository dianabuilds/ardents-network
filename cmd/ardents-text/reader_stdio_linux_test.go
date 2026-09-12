//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestTextUIDescriptorDoesNotChangeInheritedFlags(t *testing.T) {
	input, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer output.Close()
	for _, test := range []struct {
		file *os.File
		mode int
	}{{input, os.O_RDONLY}, {output, os.O_WRONLY}} {
		fd := test.file.Fd()
		before, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0)
		if errno != 0 {
			t.Fatal(errno)
		}
		opened, err := openTextPresentationDescriptor(fmt.Sprintf("/proc/self/fd/%d", fd), test.file, test.mode)
		if err != nil {
			t.Fatal(err)
		}
		if err := opened.SetDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		after, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0)
		if errno != 0 || before != after {
			t.Fatal("trusted UI changed shared inherited descriptor flags")
		}
		if err := opened.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTextUIDescriptorRejectsSubstitution(t *testing.T) {
	input, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer output.Close()
	other, otherWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	defer otherWriter.Close()
	opened, err := openTextPresentationDescriptor(fmt.Sprintf("/proc/self/fd/%d", other.Fd()), input, os.O_RDONLY)
	if opened != nil || err == nil {
		t.Fatal("UI used a substituted descriptor")
	}
}
