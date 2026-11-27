// Package targetlink reads and validates the explicit local Target Link input.
// Unix-family file opens reject symlinks and non-regular files; unsupported
// platforms refuse before opening. A Link selects no Route or authority.
package targetlink
