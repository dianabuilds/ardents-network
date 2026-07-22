//go:build windows

package storage

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/stretchr/testify/require"
)

func TestPrivateStateACLExcludesWorldAndBuiltinUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "state.json")
	require.NoError(t, AtomicWritePrivateFile(path, []byte("secret")))

	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	require.NoError(t, err)
	sddl := descriptor.String()
	require.NotContains(t, sddl, ";;;WD)")
	require.NotContains(t, sddl, ";;;BU)")
	require.Contains(t, sddl, ";;;SY)")
}

func TestPrivateDirectoryACLIsAcceptedWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity")
	require.NoError(t, EnsurePrivateDir(path))
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.NoError(t, validatePrivateDirectory(path, info))
	require.Contains(t, descriptor.String(), ";;;SY)")
}

func TestAtomicCreatePrivateFileProtectsNewParentAndFile(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "identity")
	path := filepath.Join(parent, "signer.json")
	require.NoError(t, AtomicCreatePrivateFile(path, []byte("secret")))
	require.NoError(t, ValidatePrivateDir(parent))

	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	require.NoError(t, err)
	sddl := descriptor.String()
	require.NotContains(t, sddl, ";;;WD)")
	require.NotContains(t, sddl, ";;;BU)")
	require.Contains(t, sddl, ";;;SY)")
}

func TestAtomicCreatePrivateFileRefusesAndDoesNotRewriteUnsafeExistingParent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "shared")
	require.NoError(t, os.Mkdir(parent, 0o755))
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	require.NoError(t, err)
	system, err := windows.StringToSid("S-1-5-18")
	require.NoError(t, err)
	builtinUsers, err := windows.StringToSid("S-1-5-32-545")
	require.NoError(t, err)
	entries := []windows.EXPLICIT_ACCESS{
		privateAccessEntry(user.User.Sid, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT),
		privateAccessEntry(system, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT),
		privateAccessEntry(builtinUsers, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT),
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	require.NoError(t, err)
	require.NoError(t, windows.SetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil))
	before, err := windows.GetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	require.NoError(t, err)
	original, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(parent))
	t.Cleanup(func() { require.NoError(t, os.Chdir(original)) })

	err = AtomicCreatePrivateFile("signer.json", []byte("secret"))
	require.ErrorContains(t, err, "unexpected trustees")
	after, descriptorErr := windows.GetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	require.NoError(t, descriptorErr)
	require.Equal(t, before.String(), after.String())
	_, statErr := os.Stat(filepath.Join(parent, "signer.json"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}
