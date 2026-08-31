//go:build windows

package enrollment

import "os"

func verifyOwnedRegular(os.FileInfo) error { return nil }
