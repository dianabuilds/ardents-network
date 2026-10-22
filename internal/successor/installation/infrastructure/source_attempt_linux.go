package infrastructure

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

// Attempts retain every original start and retirement. Exhaustion refuses;
// no history is pruned and no old pathname is reused to restart a Source.
const sourceAttemptLimit = 64

type sourceAttemptNumber uint32

func (n sourceAttemptNumber) directory(root string) string {
	if n == 0 {
		return filepath.Join(root, "source-startup")
	}
	return filepath.Join(root, fmt.Sprintf("source-invocation-%08d", n))
}

func (n sourceAttemptNumber) journal(root string) string {
	if n == 0 {
		return root + ".source-first-start-journal"
	}
	return root + fmt.Sprintf(".source-invocation-%08d-journal", n)
}

func (n sourceAttemptNumber) schema() (record, phase, intent, archive string) {
	if n == 0 {
		return "ardents-source-first-start-record-v1", "first-start", "ardents-source-first-start-intent-v1", "ardents-source-first-start-archive-v1"
	}
	return "ardents-source-invocation-record-v1", "invocation", "ardents-source-invocation-intent-v1", "ardents-source-invocation-archive-v1"
}

// Directory discovery chooses only which original root-peer exchange to try.
// It grants no Start, old ACK replay, clean retirement or runtime authority.
// A partial/gapped/foreign prefix refuses. Root's separate complete history
// reader must independently authenticate every predecessor before restart.
func latestSourceAttempt(selection *sourceSelection) (sourceAttemptNumber, error) {
	if selection == nil || selection.closed {
		return 0, ErrInput
	}
	root := selection.binding.Request.InstallationRoot
	directory := selection.dirs[root]
	if directory == nil {
		return 0, ErrBinding
	}
	if err := selection.observe(); err != nil {
		return 0, err
	}
	if _, err := directory.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	entries, err := directory.ReadDir(sourceAttemptLimit + 16)
	if err != nil && err != io.EOF || len(entries) >= sourceAttemptLimit+16 {
		return 0, errors.Join(ErrBinding, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name != "source-startup" && !strings.HasPrefix(name, "source-invocation-") {
			continue
		}
		if !entry.IsDir() {
			return 0, ErrBinding
		}
		names = append(names, name)
	}
	latest, err := sourceAttemptDirectories(names)
	if err != nil {
		return 0, err
	}
	return latest, selection.observe()
}

func sourceAttemptDirectories(names []string) (sourceAttemptNumber, error) {
	if len(names) == 0 || len(names) > sourceAttemptLimit {
		return 0, ErrBinding
	}
	seen := map[uint64]bool{}
	var latest uint64
	for _, name := range names {
		if name == "source-startup" {
			if seen[0] {
				return 0, ErrBinding
			}
			seen[0] = true
			continue
		}
		n, e := strconv.ParseUint(strings.TrimPrefix(name, "source-invocation-"), 10, 32)
		if e != nil || n == 0 || n >= sourceAttemptLimit || name != fmt.Sprintf("source-invocation-%08d", n) || seen[n] {
			return 0, ErrBinding
		}
		seen[n] = true
		if n > latest {
			latest = n
		}
	}
	for n := uint64(0); n <= latest; n++ {
		if !seen[n] {
			return 0, ErrBinding
		}
	}
	return sourceAttemptNumber(latest), nil
}
