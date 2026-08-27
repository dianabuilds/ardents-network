package endpoint

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

// FirefoxBrowser opens only an Endpoint-created Reference Site origin in the
// participant-selected Firefox executable. An alpha `.ard` URL additionally
// requires the separately installed Alpha Browser Entry; this adapter neither
// installs nor changes Firefox's profile, proxy, DNS, VPN, or trust store.
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

func validateFirefoxReferenceURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("firefox browser received an invalid Reference origin")
	}
	if validFirefoxAlphaHost(parsed.Hostname()) && parsed.Port() == "" && parsed.EscapedPath() == "/" {
		return nil
	}
	if !validFirefoxReferenceHost(parsed.Hostname()) || parsed.Port() == "" || !strings.HasPrefix(parsed.EscapedPath(), "/site/") {
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

func validFirefoxAlphaHost(hostname string) bool {
	const suffix = ".ard"
	if !strings.HasSuffix(hostname, suffix) || len(hostname) <= len(suffix) {
		return false
	}
	_, err := alpha.ParseServiceLink("ardents-alpha://" + strings.TrimSuffix(hostname, suffix))
	return err == nil
}

func validFirefoxReferenceHost(hostname string) bool {
	return hostname == "127.0.0.1"
}
