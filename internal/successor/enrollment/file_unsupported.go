//go:build !windows && !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package enrollment

import "os"

func verifyOwnedFile(os.FileInfo) error                 { return ErrInventory }
func openBundleFile(*os.Root, string) (*os.File, error) { return nil, ErrInventory }
