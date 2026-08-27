//go:build windows

package endpoint

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// startFirefox uses the Windows shell only to hand one validated URL to the
// explicitly selected Firefox executable. Direct CreateProcess launcher stubs
// can remain waiting while an existing Firefox profile owns its command pipe;
// ShellExecute performs the normal Windows application handoff without
// selecting a default browser or changing its configuration.
func startFirefox(executable, referenceURL string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return err
	}
	arguments, err := windows.UTF16PtrFromString(windows.EscapeArg("-new-window") + " " + windows.EscapeArg(referenceURL))
	if err != nil {
		return err
	}
	if err := windows.ShellExecute(0, verb, file, arguments, nil, windows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("open selected Firefox: %w", err)
	}
	return nil
}
