package endpoint

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// FirefoxBrowser opens only an Endpoint-created Reference Site origin in the
// participant-selected Firefox executable. It neither owns Firefox's profile
// or lifetime nor changes browser, proxy, DNS, VPN, or trust-store settings.
type FirefoxBrowser struct {
	executable string
	start      func(string, string) error
}

// NewFirefoxBrowser validates one explicit browser executable. It does not
// search PATH or select a browser on the participant's behalf.
func NewFirefoxBrowser(executable string) (*FirefoxBrowser, error) {
	if executable == "" || !filepath.IsAbs(executable) {
		return nil, errors.New("firefox executable path is invalid")
	}
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("firefox executable is unavailable")
	}
	return &FirefoxBrowser{executable: executable, start: startFirefox}, nil
}

// OpenReference implements ReferenceBrowser. Firefox remains an external,
// user-owned process, so Endpoint releases the short command request after it
// has been accepted rather than terminating an existing browser at connection
// close.
func (browser *FirefoxBrowser) OpenReference(ctx context.Context, ready ReferenceReady) error {
	if browser == nil || browser.start == nil || ctx == nil {
		return errors.New("firefox Reference browser is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateFirefoxReferenceURL(ready.URL); err != nil {
		return err
	}
	return browser.start(browser.executable, ready.URL)
}

func startFirefox(executable, referenceURL string) error {
	command := exec.Command(executable, "-new-window", referenceURL)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func validateFirefoxReferenceURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" || !strings.HasPrefix(parsed.EscapedPath(), "/site/") {
		return errors.New("firefox browser received an invalid Reference origin")
	}
	port, portErr := strconv.ParseUint(parsed.Port(), 10, 16)
	if portErr != nil || port == 0 {
		return errors.New("firefox browser received an invalid Reference origin")
	}
	parts := strings.Split(strings.TrimPrefix(parsed.EscapedPath(), "/site/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "" {
		return errors.New("firefox browser received an invalid Reference origin")
	}
	return nil
}
