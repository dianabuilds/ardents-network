// Package architecture contains executable checks for the repository's Go
// architecture. The tests fail before structural drift can enter the project.
//
// successor_import_scan_test.go checks imports in every selected source file,
// including tests and platform variants. successor_import_rules_test.go
// applies the exact permissions from the source and package owners.
// successor_package_imports_test.go records package-level edges, while
// retired_imports_test.go denies the predecessor directories and files.
// successor_import_policy_test.go exercises both permissions and refusals.
//
// Route migration has a separate import check in
// route_migration_isolation_test.go. Platform file selection is checked in
// platform_selection_test.go and route_platform_selection_test.go.
package architecture
