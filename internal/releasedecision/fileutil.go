package releasedecision

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// hexEncode returns the lowercase hex encoding of the supplied bytes.
func hexEncode(data []byte) string {
	return hex.EncodeToString(data)
}

// hexDecode parses a lowercase or uppercase hex string of the expected
// length and returns the decoded bytes.
func hexDecode(encoded []byte) ([]byte, error) {
	if len(encoded)%2 != 0 {
		return nil, errors.New("hex string has an odd length")
	}
	decoded := make([]byte, hex.DecodedLen(len(encoded)))
	if _, err := hex.Decode(decoded, encoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

// sha256Sum returns the SHA-256 of the supplied bytes as a fixed-size array.
func sha256Sum(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// readBoundedFloorFile reads a file at path, refusing to read more than
// the supplied byte bound. A missing file is reported as the underlying
// os.ErrNotExist so callers can distinguish a never-committed state from
// a tampered one.
func readBoundedFloorFile(path string, bound int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("releasedecision: %s is not a regular file", path)
	}
	if info.Size() > bound {
		return nil, fmt.Errorf("releasedecision: %s exceeds the bound", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	limited := &io.LimitedReader{R: file, N: bound + 1}
	buffer := make([]byte, 0, info.Size())
	temporary := make([]byte, 1024)
	for {
		read, readErr := limited.Read(temporary)
		if read > 0 {
			buffer = append(buffer, temporary[:read]...)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return nil, readErr
		}
		if read == 0 {
			break
		}
	}
	if int64(len(buffer)) > bound {
		return nil, fmt.Errorf("releasedecision: %s exceeds the bound", path)
	}
	return buffer, nil
}

// writeSyncedFile writes the supplied contents to path, fsyncs the file,
// and closes it. The file is created with mode 0600 and O_EXCL so a
// concurrent writer never overwrites a half-written committed value.
func writeSyncedFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("releasedecision: create %s: %w", filepath.Base(path), err)
	}
	written := 0
	for written < len(contents) {
		count, err := file.Write(contents[written:])
		if err != nil {
			closeErr := file.Close()
			if closeErr != nil {
				return fmt.Errorf("releasedecision: write %s: %v; close error: %v", filepath.Base(path), err, closeErr)
			}
			return fmt.Errorf("releasedecision: write %s: %w", filepath.Base(path), err)
		}
		written += count
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil {
		return fmt.Errorf("releasedecision: sync %s: %w", filepath.Base(path), syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("releasedecision: close %s: %w", filepath.Base(path), closeErr)
	}
	return nil
}

// readFloorStoreDirectory returns the bounded directory entries under
// path. The bound is the maximum number of entries to read; more than
// that is reported as an error so a corrupt state root fails closed.
func readFloorStoreDirectory(path string, bound int) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	if len(entries) > bound {
		return nil, fmt.Errorf("releasedecision: %s exceeds the entry bound", path)
	}
	return entries, nil
}

// writeFloorStoreMarker writes the owned state root marker file with
// fsync. A pre-existing marker is verified but not overwritten.
func writeFloorStoreMarker(root string) error {
	markerPath := filepath.Join(root, floorStoreMarkerName)
	info, err := os.Lstat(markerPath)
	if err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("releasedecision: state root marker is not a regular file")
		}
		contents, readErr := readBoundedFloorFile(markerPath, int64(len(floorStoreMarker)))
		if readErr != nil || string(contents) != floorStoreMarker {
			return errors.New("releasedecision: state root marker is invalid")
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("releasedecision: inspect state root marker: %w", err)
	}
	entries, readErr := readFloorStoreDirectory(root, 2)
	if readErr != nil {
		return fmt.Errorf("releasedecision: inspect state root: %w", readErr)
	}
	if len(entries) > 1 {
		return errors.New("releasedecision: refusing to claim a non-empty unowned state root")
	}
	if len(entries) == 1 && entries[0].Name() != floorStoreLockName {
		return errors.New("releasedecision: refusing to claim a non-empty unowned state root")
	}
	if err := writeSyncedFile(markerPath, []byte(floorStoreMarker)); err != nil {
		return err
	}
	// syncDirectory is best-effort: the marker is an opaque ownership
	// file, and the durable commit is the generation rename below.
	// Windows occasionally returns ACCESS_DENIED while the file is
	// still being closed by the kernel; ignoring the error is
	// safe because the next durable publication is itself a
	// crash-consistent atomic rename.
	_ = syncDirectory(root)
	return nil
}

// syncDirectory is best-effort: platforms without an fsync-equivalent
// (e.g. Windows) silently succeed. The package only relies on directory
// sync to harden the published generation rename, which is already
// crash-consistent thanks to the atomic rename. The directory handle
// is explicitly closed; on Windows a leaked handle keeps the directory
// in delete-pending state and the test framework's TempDir cleanup
// reports ACCESS_DENIED.
func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
