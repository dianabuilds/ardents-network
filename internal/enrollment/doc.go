// Package enrollment owns the closed-alpha first-artifact check. It compares an
// independently delivered manifest pin before parsing static content, verifies
// the exact inventory, descriptor, and either a bundled or one explicit
// package-owned executable, then prepares the same bytes for Release Decision.
// It is not a general downloader, updater, installer, or release authority.
package enrollment
