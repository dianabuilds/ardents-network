package node

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func (observer *nodeObserver) result(verdict string, cause error) Result {
	observer.cleanup()
	if observer.cleanupErr != nil {
		verdict, cause = "fail", errors.Join(cause, observer.cleanupErr)
	}
	observer.recordEvidenceError(observer.samples.Sync())
	observer.recordEvidenceError(observer.samples.Close())
	observer.mu.Lock()
	evidenceErr := observer.evidenceErr
	observer.mu.Unlock()
	if evidenceErr != nil {
		verdict, cause = "invalid", errors.Join(cause, errors.New("node external evidence failed"), evidenceErr)
	}
	if verdict == "pass" {
		if resourceErr := observer.verifyResourceEvidence(); resourceErr != nil {
			verdict, cause = "fail", errors.Join(cause, resourceErr)
		}
	}
	digest, digestErr := nodeEvidenceDigest(observer.input.EvidenceRoot)
	if digestErr != nil {
		verdict, cause = "invalid", errors.Join(cause, digestErr)
	}
	reason := boundedNodeReason(cause)
	result := Result{Verdict: verdict, Reason: reason, EvidenceRoot: observer.input.EvidenceRoot, EvidenceDigest: digest}
	if err := writeNodeResult(observer.input.EvidenceRoot, result); err != nil {
		result.Verdict = "invalid"
		result.Reason = boundedNodeReason(errors.Join(errors.New("node terminal result was not durable"), err))
	}
	return result
}

func boundedNodeReason(cause error) string {
	if cause == nil {
		return "node campaign passed"
	}
	reason := cause.Error()
	if len(reason) > 4096 {
		reason = reason[:4096]
	}
	return reason
}

func (observer *nodeObserver) recordEvidenceError(err error) {
	if err == nil {
		return
	}
	observer.mu.Lock()
	if observer.evidenceErr == nil {
		observer.evidenceErr = err
	}
	observer.mu.Unlock()
	observer.evidenceOnce.Do(func() { close(observer.evidenceBad) })
}

func writeNodeResult(root string, result Result) error {
	raw, err := json.Marshal(result)
	if err != nil || len(raw) > 64<<10 {
		return errors.Join(err, errors.New("node terminal result exceeds its bound"))
	}
	temporary := filepath.Join(root, ".result-stage")
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(append(raw, '\n'))
	if writeErr == nil && written != len(raw)+1 {
		writeErr = io.ErrShortWrite
	}
	writeErr = errors.Join(writeErr, file.Sync(), file.Close())
	if writeErr != nil {
		_ = os.Remove(temporary)
		return writeErr
	}
	if err := os.Rename(temporary, filepath.Join(root, "result.json")); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return syncNodeDirectory(root)
}

func nodeEvidenceDigest(root string) (string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr == nil && !entry.IsDir() && entry.Name() != "result.json" {
			paths = append(paths, path)
		}
		return walkErr
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		file, openErr := os.Open(path)
		if openErr != nil {
			return "", openErr
		}
		fileHash := sha256.New()
		_, copyErr := io.Copy(fileHash, file)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return "", errors.Join(copyErr, closeErr)
		}
		relative, _ := filepath.Rel(root, path)
		_, _ = hash.Write([]byte(filepath.ToSlash(relative) + "\x00" + hex.EncodeToString(fileHash.Sum(nil)) + "\n"))
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
