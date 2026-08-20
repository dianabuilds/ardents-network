//go:build ignore

package r050

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

const maximumWindowsPathUnits = 240

func validatePlatformRoot(root string) error {
	if len(utf16.Encode([]rune(filepath.Join(root, activationFile)))) > maximumWindowsPathUnits {
		return fmt.Errorf("Windows path exceeds %d UTF-16 units: %w", maximumWindowsPathUnits, errActivationUnsupported)
	}
	if err := rejectWindowsReparse(root); err != nil {
		return err
	}
	filesystem, _, drive, err := windowsVolume(root)
	if err != nil {
		return err
	}
	if filesystem != "NTFS" || drive != windows.DRIVE_FIXED {
		return fmt.Errorf("filesystem=%s drive=%d: %w", filesystem, drive, errActivationUnsupported)
	}
	return validateWindowsSecurity(root, true)
}

func platformSecureTemporary(path string) error {
	return validateWindowsSecurity(path, false)
}

func validatePlatformTemporary(file *os.File) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return fmt.Errorf("temporary handle info: %w", err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.NumberOfLinks != 1 {
		return fmt.Errorf("unsafe activation temp attributes=%#x links=%d: %w", info.FileAttributes, info.NumberOfLinks, errActivationUnsupported)
	}
	return validateWindowsSecurity(file.Name(), false)
}

func platformReplace(from, to string) error {
	fromUTF16, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	toUTF16, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	err = windows.MoveFileEx(fromUTF16, toUTF16, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return fmt.Errorf("replace activation (%v): %w", err, errActivationBusy)
	}
	if err != nil {
		return fmt.Errorf("replace activation: %w", err)
	}
	return nil
}

func platformSyncParent(string) error {
	// MoveFileExW WRITE_THROUGH is the candidate Windows completion primitive.
	// R-050 does not infer power-loss safety from this smoke call.
	return nil
}

func validatePlatformCommitted(path string) error {
	pathUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(pathUTF16, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 || info.NumberOfLinks != 1 {
		return fmt.Errorf("unsafe committed activation attributes=%#x links=%d: %w", info.FileAttributes, info.NumberOfLinks, errActivationUnsupported)
	}
	return validateWindowsSecurity(path, false)
}

func platformReadActivation(path string) ([]byte, error) {
	pathUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		pathUTF16,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	defer file.Close()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return nil, err
	}
	size := uint64(info.FileSizeHigh)<<32 | uint64(info.FileSizeLow)
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.NumberOfLinks != 1 || size > maximumActivationLen {
		return nil, fmt.Errorf("unsafe activation read handle: %w", errActivationUnsupported)
	}
	return io.ReadAll(io.LimitReader(file, maximumActivationLen+1))
}

func platformManifest(root string) (string, error) {
	filesystem, serial, drive, err := windowsVolume(root)
	if err != nil {
		return "", err
	}
	version := windows.RtlGetVersion()
	return fmt.Sprintf("windows=%d.%d.%d filesystem=%s volume_serial=%08x drive_type=%d", version.MajorVersion, version.MinorVersion, version.BuildNumber, filesystem, serial, drive), nil
}

func windowsVolume(path string) (string, uint32, uint32, error) {
	pathUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", 0, 0, err
	}
	root := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(pathUTF16, &root[0], uint32(len(root))); err != nil {
		return "", 0, 0, err
	}
	filesystem := make([]uint16, 32)
	var serial uint32
	if err := windows.GetVolumeInformation(&root[0], nil, 0, &serial, nil, nil, &filesystem[0], uint32(len(filesystem))); err != nil {
		return "", 0, 0, err
	}
	return windows.UTF16ToString(filesystem), serial, windows.GetDriveType(&root[0]), nil
}

func validateWindowsSecurity(path string, requireProtected bool) error {
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("security descriptor %s: %w", path, err)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		return err
	}
	if requireProtected && control&windows.SE_DACL_PROTECTED == 0 {
		return fmt.Errorf("DACL is inherited: %w", errActivationUnsupported)
	}
	dacl, _, err := descriptor.DACL()
	wantACECount := uint16(1)
	if requireProtected {
		wantACECount = 2
	}
	if err != nil || dacl == nil || dacl.AceCount != wantACECount {
		count := uint16(0)
		if dacl != nil {
			count = dacl.AceCount
		}
		return fmt.Errorf("DACL ACE count=%d want=%d descriptor=%s: %w", count, wantACECount, descriptor.String(), errActivationUnsupported)
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || !owner.Equals(user.User.Sid) {
		return fmt.Errorf("owner SID mismatch: %w", errActivationUnsupported)
	}
	sddl := descriptor.String()
	if !strings.Contains(sddl, ";;;"+user.User.Sid.String()+")") || strings.Contains(sddl, ";;;SY)") {
		return fmt.Errorf("DACL trustees mismatch: %w", errActivationUnsupported)
	}
	return nil
}

func rejectWindowsReparse(path string) error {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	current := volume + string(filepath.Separator)
	rest := strings.TrimPrefix(clean[len(volume):], string(filepath.Separator))
	for _, component := range strings.Split(rest, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		attributes, err := windows.GetFileAttributes(windows.StringToUTF16Ptr(current))
		if err != nil {
			return err
		}
		if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("reparse component %s: %w", current, errActivationUnsupported)
		}
	}
	return nil
}
