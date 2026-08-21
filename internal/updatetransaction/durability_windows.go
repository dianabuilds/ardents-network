//go:build windows

package updatetransaction

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

type durabilityOps struct {
	replaceCurrent    func(string, string) error
	publishGeneration func(string, string) error
	syncDirectory     func(string) error
}

func nativeDurability() durabilityOps {
	return durabilityOps{
		replaceCurrent:    windowsReplaceCurrent,
		publishGeneration: windowsPublishGeneration,
		syncDirectory:     windowsSyncDirectory,
	}
}

func windowsReplaceCurrent(temporary, current string) error {
	return windowsMove(temporary, current, windows.MOVEFILE_REPLACE_EXISTING)
}

func windowsPublishGeneration(staging, generation string) error {
	if _, err := os.Lstat(generation); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return errors.New("generation already exists")
		}
		return fmt.Errorf("inspect generation target: %w", err)
	}
	info, err := os.Lstat(staging)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.Join(errors.New("staging is not a direct directory"), err)
	}
	return windowsMove(staging, generation, 0)
}

func windowsMove(source, target string, flags uint32) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	if err := windows.MoveFileEx(from, to, flags|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return fmt.Errorf("durable move: %w", err)
	}
	return nil
}

func windowsSyncDirectory(string) error {
	// File handles are flushed before publication, and both Windows publication
	// primitives use MOVEFILE_WRITE_THROUGH. Windows exposes no directory fsync.
	return nil
}

func validateOwnedPath(path string) error {
	volume := filepath.VolumeName(path)
	if volume == "" || filepath.Clean(path) != path || len(utf16.Encode([]rune(path))) > 240 {
		return errors.New("owned path is invalid")
	}
	current := volume + string(filepath.Separator)
	remaining := strings.TrimPrefix(path[len(volume):], string(filepath.Separator))
	for _, component := range strings.Split(remaining, string(filepath.Separator)) {
		if len(component) == 0 || len(component) > 64 {
			return errors.New("owned path component is invalid")
		}
		current = filepath.Join(current, component)
		encoded, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return err
		}
		attributes, err := windows.GetFileAttributes(encoded)
		if err != nil {
			return err
		}
		if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return errors.New("owned path crosses a reparse point")
		}
	}
	return nil
}

func validateOwnedEntry(path string) error {
	encoded, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes, err := windows.GetFileAttributes(encoded)
	if err != nil || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.Join(errRecordInvalid, err)
	}
	if attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return nil
	}
	handle, err := windows.CreateFile(encoded, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	var information windows.ByHandleFileInformation
	readErr := windows.GetFileInformationByHandle(handle, &information)
	if readErr == nil && information.NumberOfLinks != 1 {
		readErr = errRecordInvalid
	}
	return errors.Join(readErr, windows.CloseHandle(handle))
}
