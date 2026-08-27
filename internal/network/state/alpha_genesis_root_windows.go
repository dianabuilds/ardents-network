//go:build windows

package state

import (
	"errors"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func validateAlphaGenesisRootAccess(root string, _ os.FileInfo) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return errors.New("read functional alpha State owner SID")
	}
	descriptor, err := windows.GetNamedSecurityInfo(root, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || descriptor == nil {
		return errors.New("read functional alpha State root security")
	}
	owner, _, err := descriptor.Owner()
	if err != nil || owner == nil || !owner.Equals(user.User.Sid) {
		return errors.New("functional alpha State root is not owned by the current user")
	}
	dacl, defaulted, err := descriptor.DACL()
	if err != nil || dacl == nil || defaulted || dacl.AceCount == 0 {
		return errors.New("functional alpha State root DACL is unavailable")
	}
	if strings.Count(descriptor.String(), ";;;"+user.User.Sid.String()+")") != int(dacl.AceCount) {
		return errors.New("functional alpha State root DACL contains another trustee")
	}
	return nil
}
