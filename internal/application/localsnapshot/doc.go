// Package localsnapshot reads an explicitly selected local file for publication.
// On Linux and macOS it opens every path component without following symlinks
// and verifies the bounded contents before transferring a copy to the caller.
// Other platforms refuse the import. The caller clears the copy after transfer.
package localsnapshot
