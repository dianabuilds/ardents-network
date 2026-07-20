//go:build windows

package execution

import "golang.org/x/sys/windows"

func ProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	result, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return false
	}
	return result == uint32(windows.WAIT_TIMEOUT)
}
