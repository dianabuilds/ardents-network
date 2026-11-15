package introduction

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
)

// The closed initial-generation profile retains every attempt, never recycling
// a pathname or deleting history on exhaustion. This is an Installation read
// bound, not another network retry, token or Hosting allowance.
const historyDirectoryLimit = 2*attemptLimit + 16

type attemptNumber uint32

func (n attemptNumber) directory(root string) string {
	if n >= attemptLimit {
		return ""
	}
	if n == 0 {
		return filepath.Join(root, "introduction-startup")
	}
	return filepath.Join(root, fmt.Sprintf("introduction-invocation-%08d", n))
}

func (n attemptNumber) journal(root string) string {
	if n >= attemptLimit {
		return ""
	}
	if n == 0 {
		return root + ".introduction-first-start-journal"
	}
	return root + fmt.Sprintf(".introduction-invocation-%08d-journal", n)
}

func (n attemptNumber) retirement(root string) string {
	if n >= attemptLimit {
		return ""
	}
	return filepath.Join(root, fmt.Sprintf("introduction-retirement-%08d", n))
}

func (n attemptNumber) retirementJournal(root string) string {
	if n >= attemptLimit {
		return ""
	}
	return root + fmt.Sprintf(".introduction-retirement-%08d-journal", n)
}

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
		if name != "introduction-startup" && !strings.HasPrefix(name, "introduction-invocation-") {
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
	number, err := journal.InvocationPrefix(names, "introduction", attemptLimit)
	if err != nil {
		return 0, ErrBinding
	}
	return attemptNumber(number), nil
}
