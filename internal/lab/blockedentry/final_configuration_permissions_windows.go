//go:build windows

package blockedentry

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func validateFinalConfigurationTree(root string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return errors.New("read final configuration owner SID")
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.Type()&fs.ModeSymlink != 0 {
			return errors.Join(walkErr, errors.New("final configuration tree is unavailable or aliased"))
		}
		info, err := entry.Info()
		if err != nil || !entry.IsDir() && (!info.Mode().IsRegular() || info.Mode().Perm()&0o222 != 0) {
			return errors.Join(err, errors.New("final configuration entry is mutable or invalid"))
		}
		descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
			windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
		if err != nil || descriptor == nil {
			return errors.Join(err, errors.New("final configuration security descriptor is unavailable"))
		}
		owner, _, ownerErr := descriptor.Owner()
		control, _, controlErr := descriptor.Control()
		sddl := descriptor.String()
		if ownerErr != nil || controlErr != nil || owner == nil || !owner.Equals(user.User.Sid) ||
			control&windows.SE_DACL_PROTECTED == 0 || strings.Count(sddl, "(A;") != 1 ||
			!strings.Contains(sddl, user.User.Sid.String()) {
			return errors.Join(ownerErr, controlErr, errors.New("final configuration ACL is not protected owner-only"))
		}
		return nil
	})
}
