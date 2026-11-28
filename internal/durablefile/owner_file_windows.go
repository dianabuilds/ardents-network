//go:build windows

package durablefile

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"

	"golang.org/x/sys/windows"
)

func ownerOnlyAttributes() (*windows.SecurityAttributes, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	if user == nil || user.User.Sid == nil {
		return nil, errors.New("current user SID is unavailable")
	}
	sid := user.User.Sid.String()
	descriptor, err := windows.SecurityDescriptorFromString("O:" + sid + "D:P(A;;GA;;;" + sid + ")")
	if err != nil {
		return nil, err
	}
	attributes := &windows.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(reflect.TypeOf(*attributes).Size())
	return attributes, nil
}

func createOwnerOnlyFile(path string) (*os.File, error) {
	attributes, err := ownerOnlyAttributes()
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, attributes, windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		return nil, errors.Join(errors.New("create owner-only file"), windows.CloseHandle(handle))
	}
	return file, nil
}

// CreateOwnerOnlyTemp creates a private temporary file in the given directory.
func CreateOwnerOnlyTemp(root, prefix string) (*os.File, error) {
	for range 32 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		path := filepath.Join(root, prefix+hex.EncodeToString(random[:]))
		file, err := createOwnerOnlyFile(path)
		if errors.Is(err, windows.ERROR_FILE_EXISTS) || errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			continue
		}
		return file, err
	}
	return nil, errors.New("temporary file name attempts exhausted")
}

// OpenOwnerOnlyLease opens a fixed-name, non-sharing lease file on Windows.
// On creation it sets the owner and DACL before the file becomes visible.
func OpenOwnerOnlyLease(path string, disposition uint32) (windows.Handle, error) {
	attributes, err := ownerOnlyAttributes()
	if err != nil {
		return 0, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0,
		attributes, disposition, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
}
