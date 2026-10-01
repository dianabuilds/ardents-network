//go:build linux

package installation

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

type replacementRecord struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	Path             string `json:"path"`
	Device           uint64 `json:"device"`
	Inode            uint64 `json:"inode"`
	Mode             uint32 `json:"mode"`
	GID              uint32 `json:"gid"`
	PreviousDigest   string `json:"previous_digest"`
	CandidateDigest  string `json:"candidate_digest"`
}

// replaceTransitionFile retains the direct installed inode. Before its first
// truncation a root-only durable record binds that inode and both complete
// generation digests. An interrupted write can only be repaired explicitly on
// that same owned inode, after new Release authentication and joined scopes.
// A record is observation/repair provenance, never activation authorization.
func replaceTransitionFile(ctx context.Context, path string, previous, candidate []byte, mode os.FileMode, gid uint32, recordPath string, selected selection) (returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 || len(previous) == 0 || len(candidate) == 0 || len(candidate) > 64<<20 {
		return errors.New("successor replacement input is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkRootAncestors(filepath.Dir(path), false); err != nil {
		return err
	}
	if err := checkRootAncestors(filepath.Dir(recordPath), false); err != nil {
		return err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return err
	}
	identity, ok := before.Sys().(*syscall.Stat_t)
	if !ok || !before.Mode().IsRegular() || identity.Uid != 0 || identity.Gid != gid || identity.Nlink != 1 || before.Mode().Perm() != mode {
		return errors.New("successor replacement file ownership differs")
	}
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), path)
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return errors.New("successor replacement inode changed")
	}
	record := replacementRecord{Schema: "ardents-endpoint-installation-replacement-v1", GenerationDigest: selected.GenerationDigest,
		Path: path, Device: uint64(identity.Dev), Inode: identity.Ino, Mode: uint32(mode), GID: gid,
		PreviousDigest: digestHex(previous), CandidateDigest: digestHex(candidate)}
	recordBytes, err := canonicalJSON(record)
	if err != nil {
		return err
	}
	if existing, err := readInstalledFile(recordPath, 4096); err == nil {
		if err := requirePrivateJournalFile(recordPath); err != nil {
			return err
		}
		var observed replacementRecord
		if err := decodeCanonical(existing, 4096, &observed); err != nil || observed != record {
			return errors.New("successor replacement journal identity differs")
		}
	} else if os.IsNotExist(err) {
		body, err := io.ReadAll(io.LimitReader(file, 64<<20+1))
		if err != nil || len(body) > 64<<20 || digestHex(body) != record.PreviousDigest {
			return errors.New("successor replacement preimage differs")
		}
		if err := writeExclusiveGenerationFile(recordPath, recordBytes, 0600, 0); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(recordPath)); err != nil {
			return err
		}
	} else {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := file.Write(candidate); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	return ctx.Err()
}
