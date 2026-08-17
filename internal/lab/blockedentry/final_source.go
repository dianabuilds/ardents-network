package blockedentry

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func materializeCommittedSource(workspace string) (commit, digest, root, temporary string, returnErr error) {
	if _, err := boundedReceiptCommand("git", "-C", workspace, "diff", "--quiet", "HEAD", "--"); err != nil {
		return "", "", "", "", errors.New("tracked workspace changes must be committed before final preparation")
	}
	raw, err := boundedReceiptCommand("git", "-C", workspace, "rev-parse", "HEAD")
	commit = strings.TrimSpace(string(raw))
	if err != nil || !hexDigest(commit, 20) {
		return "", "", "", "", errors.Join(err, errors.New("repository commit identity is unavailable"))
	}
	temporary, err = os.MkdirTemp("", "ardents-stage5-source-")
	if err != nil {
		return "", "", "", "", err
	}
	complete := false
	defer func() {
		if !complete {
			returnErr = errors.Join(returnErr, os.RemoveAll(temporary))
		}
	}()
	if err := protectEvidenceTree(temporary); err != nil {
		return "", "", "", "", err
	}
	archivePath := filepath.Join(temporary, "source.tar")
	if err := writeCommittedArchive(workspace, archivePath); err != nil {
		return "", "", "", "", err
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		return "", "", "", "", err
	}
	digestHash := sha256.New()
	if _, err := io.Copy(digestHash, archive); err != nil {
		_ = archive.Close()
		return "", "", "", "", err
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		_ = archive.Close()
		return "", "", "", "", err
	}
	root = filepath.Join(temporary, "source")
	if err := os.Mkdir(root, 0o700); err != nil {
		_ = archive.Close()
		return "", "", "", "", err
	}
	err = extractCommittedArchive(archive, root)
	closeErr := archive.Close()
	if err != nil || closeErr != nil {
		return "", "", "", "", errors.Join(err, closeErr)
	}
	digest = hex.EncodeToString(digestHash.Sum(nil))
	if err := protectEvidenceTree(temporary); err != nil {
		return "", "", "", "", err
	}
	complete = true
	return commit, digest, root, temporary, nil
}

func writeCommittedArchive(workspace, target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "git", "-C", workspace, "archive", "--format=tar", "HEAD")
	prepareReceiptProcess(command)
	command.Cancel = func() error { return terminateReceiptProcess(command) }
	command.WaitDelay = 5 * time.Second
	command.Stderr = io.Discard
	stdout, pipeErr := command.StdoutPipe()
	if pipeErr != nil {
		return errors.Join(pipeErr, output.Close())
	}
	if err := command.Start(); err != nil {
		return errors.Join(err, output.Close())
	}
	written, overflow, copyErr := copyBoundedArchive(output, stdout, 1<<31)
	if overflow || copyErr != nil {
		_ = terminateReceiptProcess(command)
	}
	runErr := command.Wait()
	syncErr, closeErr := output.Sync(), output.Close()
	info, statErr := os.Stat(target)
	if ctx.Err() != nil || overflow || copyErr != nil || runErr != nil || syncErr != nil || closeErr != nil ||
		statErr != nil || info.Size() < 1 || info.Size() != written {
		return errors.Join(copyErr, runErr, syncErr, closeErr, statErr,
			errors.New("committed source archive is unavailable or oversized"))
	}
	return nil
}

func copyBoundedArchive(output io.Writer, input io.Reader, limit int64) (int64, bool, error) {
	written, err := io.CopyN(output, input, limit+1)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return written, written > limit, err
}

func extractCommittedArchive(input io.Reader, root string) error {
	reader := tar.NewReader(input)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		clean := filepath.Clean(filepath.FromSlash(header.Name))
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return errors.New("committed source archive contains an invalid path")
		}
		target := filepath.Join(root, clean)
		switch header.Typeflag {
		case tar.TypeXGlobalHeader:
			continue
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			mode := os.FileMode(0o600)
			if header.Mode&0o100 != 0 {
				mode = 0o700
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			closeErr := file.Close()
			if copyErr != nil || closeErr != nil {
				return errors.Join(copyErr, closeErr)
			}
		default:
			return errors.New("committed source archive contains a non-regular entry")
		}
	}
}
