// Package enrollment owns the closed-alpha first-bundle check. It compares an
// independently delivered manifest pin before parsing bundle content, verifies
// the exact inventory and descriptor, and prepares the same bytes for Release
// Decision. It is not a general downloader, updater, or release authority.
package enrollment
