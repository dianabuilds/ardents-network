//go:build windows

package storage

import (
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
