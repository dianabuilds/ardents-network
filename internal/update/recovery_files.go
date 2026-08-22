package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
)

const (
	maximumRootEntries        = 8
	maximumGenerationEntries  = 3
	maximumStagingEntries     = 2
	maximumTransactionEntries = 2
	maximumJournalEntries     = 10
	maximumPayloadEntries     = 3
)

// recoveryReadFile returns one bounded snapshot from a held, no-follow handle.
// Platform code proves that the path names the same direct single-link regular
// file both after open and immediately after the read.
func recoveryReadFile(path string, maximum int64) (rawFile, error) {
	var raw rawFile
	file, err := recoveryOpen(path, false)
	if err != nil {
		return raw, err
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maximum {
		return raw, errors.Join(errInventoryInvalid, statErr, file.Close())
	}
	raw = rawFile{Name: info.Name(), Size: info.Size(), IsRegular: true, IsDirect: true}
	raw.Bytes = make([]byte, int(info.Size()))
	_, readErr := io.ReadFull(file, raw.Bytes)
	revalidateErr := recoveryRevalidate(file, path, false, info.Size())
	closeErr := file.Close()
	if err := errors.Join(readErr, revalidateErr, closeErr); err != nil {
		return rawFile{}, err
	}
	return raw, nil
}

// recoveryReadDir bounds allocation before interpreting names and revalidates
// the held directory handle after enumeration. Returned entries are sorted so
// every later plan is deterministic on every supported filesystem.
func recoveryReadDir(path string, maximum int) ([]os.DirEntry, error) {
	file, err := recoveryOpen(path, true)
	if err != nil {
		return nil, err
	}
	entries, readErr := file.ReadDir(maximum + 1)
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	if len(entries) > maximum {
		readErr = fmt.Errorf("%w: too many entries in %s", errInventoryInvalid, path)
	}
	revalidateErr := recoveryRevalidate(file, path, true, -1)
	closeErr := file.Close()
	if err := errors.Join(readErr, revalidateErr, closeErr); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}
