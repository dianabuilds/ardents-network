//go:build linux

package node

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func runNodeDiskWrapper() Result {
	const seed, target = "/run/ardents/seed", "/run/ardents/state"
	files, total := 0, int64(0)
	err := filepath.WalkDir(seed, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(seed, path)
		if err != nil || relative == ".ardents-network-state-lock" {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("disk-full seed contains a link")
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		files++
		if files > 512 {
			return errors.New("disk-full seed has too many files")
		}
		written, err := copyNodeDiskFile(path, destination)
		if written > 3<<20 {
			return errors.New("disk-full seed file exceeds its byte bound")
		}
		total += written
		if total > 8<<20 {
			return errors.New("disk-full seed exceeds its byte bound")
		}
		return err
	})
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	fill, err := os.OpenFile(filepath.Join(target, "disk-pressure.bin"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	block := make([]byte, 64<<10)
	for total = 0; total <= 8<<20; total += int64(len(block)) {
		if _, err = fill.Write(block); err != nil {
			break
		}
	}
	_ = fill.Close()
	if !errors.Is(err, syscall.ENOSPC) {
		return Result{Verdict: "invalid", Reason: "disk-full tmpfs did not reach ENOSPC"}
	}
	if _, err := os.Stdout.WriteString(nodeDiskFullStimulus + "\n"); err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	command := []string{"/usr/local/bin/ardents-node", "node", "--config", "/run/ardents/config.json"}
	if err := syscall.Exec(command[0], command, os.Environ()); err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	return Result{Verdict: "invalid", Reason: "disk-full product exec returned"}
}

func copyNodeDiskFile(source, destination string) (int64, error) {
	input, err := os.Open(source)
	if err != nil {
		return 0, err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	written, copyErr := io.Copy(output, io.LimitReader(input, (3<<20)+1))
	return written, errors.Join(copyErr, output.Close())
}
