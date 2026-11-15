package forwarding

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
)

// Discovery chooses only an original root-peer exchange. It supplies no Start,
// archive, ACK replay, predecessor retirement or runtime authority.
func latestAttempt(selection *runtimeSelection) (attemptNumber, error) {
	if selection == nil || selection.files == nil {
		return 0, ErrInput
	}
	if err := selection.check(); err != nil {
		return 0, err
	}
	directory := selection.files.dirs[selection.binding.Request.InstallationRoot]
	if directory == nil {
		return 0, ErrBinding
	}
	if _, err := directory.file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	entries, err := directory.file.ReadDir(historyDirectoryLimit)
	if err != nil && err != io.EOF || len(entries) >= historyDirectoryLimit {
		return 0, errors.Join(ErrBinding, err)
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if name != "forwarding-startup" && !strings.HasPrefix(name, "forwarding-invocation-") {
			continue
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return 0, ErrBinding
		}
		names = append(names, name)
	}
	number, err := attemptDirectories(names)
	if err != nil {
		return 0, err
	}
	return number, selection.check()
}

func attemptDirectories(names []string) (attemptNumber, error) {
	number, err := journal.InvocationPrefix(names, "forwarding", attemptLimit)
	if err != nil {
		return 0, ErrBinding
	}
	return attemptNumber(number), nil
}
