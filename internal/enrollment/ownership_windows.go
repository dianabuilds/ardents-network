//go:build windows

package enrollment

import "os"

// Windows is an explicitly non-gating companion: its portable ownership
// policy is enforced by the owner-only state roots, while the Ubuntu release
// gate additionally requires Unix ownership, mode, and link checks.
func verifyOwnedRegular(info os.FileInfo) error { return nil }

func verifyPackageFile(info os.FileInfo) error { return nil }

func verifyPackageDirectory(info os.FileInfo) error { return nil }
