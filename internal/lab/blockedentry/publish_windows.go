//go:build windows

package blockedentry

import "golang.org/x/sys/windows"

func publishDirectory(source, target string) error {
	return windows.MoveFile(windows.StringToUTF16Ptr(source), windows.StringToUTF16Ptr(target))
}
