package admission

import "runtime"

// SupportsOfflineOperatorCommands reports whether the closed operator profile
// admits local mutations on this host. Store support is checked separately.
func SupportsOfflineOperatorCommands() bool {
	return runtime.GOOS == "linux" || runtime.GOOS == "darwin"
}
