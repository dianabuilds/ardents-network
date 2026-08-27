// Package installer owns the per-user Firefox native-manifest lifecycle for
// the fixed alpha Browser Entry host. It verifies only the fixed extension ID,
// installs/removes only its exact native-manifest registration, and never
// installs an XPI, edits browser proxy settings, or selects an Endpoint state
// path.
package installer
