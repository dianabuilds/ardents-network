package forwarding

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
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
	if len(names) == 0 || len(names) > attemptLimit {
		return 0, ErrBinding
	}
	seen := map[uint64]bool{}
	var latest uint64
	for _, name := range names {
		var number uint64
		if name != "forwarding-startup" {
			n, err := strconv.ParseUint(strings.TrimPrefix(name, "forwarding-invocation-"), 10, 32)
			if err != nil || n == 0 || n >= attemptLimit || name != fmt.Sprintf("forwarding-invocation-%08d", n) {
				return 0, ErrBinding
			}
			number = n
		}
		if seen[number] {
			return 0, ErrBinding
		}
		seen[number] = true
		latest = max(latest, number)
	}
	for n := uint64(0); n <= latest; n++ {
		if !seen[n] {
			return 0, ErrBinding
		}
	}
	return attemptNumber(latest), nil
}
