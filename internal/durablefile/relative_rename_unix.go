//go:build linux || darwin

package durablefile

import "golang.org/x/sys/unix"

// Renameat atomically renames one entry within or between retained directory
// descriptors without resolving either name against a pathname that could now
// designate a replacement root. It is the Unix-family (Linux and macOS)
// spelling of the directory-relative rename the durable-store staged
// replacement idiom uses; the caller keeps every flag and ordering decision and
// owns both directory descriptors. The reviewed x/sys/unix dependency is
// admitted only here.
func Renameat(olddirfd int, oldname string, newdirfd int, newname string) error {
	return unix.Renameat(olddirfd, oldname, newdirfd, newname)
}
