//go:build windows

package durablefile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"

	"golang.org/x/sys/windows"
)

func createOwnedRoot(root string) error {
	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		return err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	if user == nil || user.User.Sid == nil {
		return errors.New("current user SID is unavailable")
	}
	sid := user.User.Sid.String()
	descriptor, err := windows.SecurityDescriptorFromString("O:" + sid + "D:P(A;OICI;GA;;;" + sid + ")")
	if err != nil {
		return err
	}
	attributes := windows.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(reflect.TypeOf(attributes).Size())
	name, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return err
	}
	return windows.CreateDirectory(name, &attributes)
}
