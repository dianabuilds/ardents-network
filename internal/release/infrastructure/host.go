package infrastructure

import "runtime"

// CommandHostSupported reports the host supported by the native artifact flow.
func CommandHostSupported() bool { return runtime.GOOS == "linux" && runtime.GOARCH == "amd64" }
