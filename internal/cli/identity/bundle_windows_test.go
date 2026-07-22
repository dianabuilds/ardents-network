//go:build windows

package identity

import (
	"bytes"
	"crypto/ed25519"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/stretchr/testify/require"
)

func TestSignerReadRefusesUnsafeWindowsFileACLWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity", "principal.json")
	_, err := CreatePrincipal(path, bytes.NewReader(bytes.Repeat([]byte{0xb1}, ed25519.SeedSize)))
	require.NoError(t, err)

	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	require.NoError(t, err)
	system, err := windows.StringToSid("S-1-5-18")
	require.NoError(t, err)
	builtinUsers, err := windows.StringToSid("S-1-5-32-545")
	require.NoError(t, err)
	entries := []windows.EXPLICIT_ACCESS{
		windowsPrivateAccessEntry(user.User.Sid),
		windowsPrivateAccessEntry(system),
		windowsPrivateAccessEntry(builtinUsers),
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	require.NoError(t, err)
	require.NoError(t, windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil))
	before, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	require.NoError(t, err)

	_, err = ShowPrincipal(path)
	require.ErrorIs(t, err, ErrSignerFileUnsafe)
	after, descriptorErr := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	require.NoError(t, descriptorErr)
	require.Equal(t, before.String(), after.String())
}

func windowsPrivateAccessEntry(sid *windows.SID) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}
