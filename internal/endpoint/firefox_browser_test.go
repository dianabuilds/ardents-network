package endpoint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFirefoxBrowserOpensOnlyScopedReferenceOrigin(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "firefox.exe")
	if err := os.WriteFile(executable, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	browser, err := NewFirefoxBrowser(executable)
	if err != nil {
		t.Fatal(err)
	}
	var openedExecutable, openedURL string
	browser.start = func(executable, referenceURL string) error {
		openedExecutable, openedURL = executable, referenceURL
		return nil
	}
	url := "http://127.0.0.1:42101/site/5d6c4df9a45a6ec7f88a0dc9b95d488d/"
	if err := browser.OpenReference(context.Background(), ReferenceReady{URL: url}); err != nil {
		t.Fatal(err)
	}
	if openedExecutable != executable || openedURL != url {
		t.Fatalf("Firefox open = %q %q", openedExecutable, openedURL)
	}
}

func TestFirefoxBrowserRejectsAnythingExceptOneScopedReferenceOrigin(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "firefox.exe")
	if err := os.WriteFile(executable, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	browser, err := NewFirefoxBrowser(executable)
	if err != nil {
		t.Fatal(err)
	}
	opened := false
	browser.start = func(string, string) error { opened = true; return nil }
	for _, value := range []string{
		"https://127.0.0.1:42101/site/opaque/", "http://localhost:42101/site/opaque/", "http://127.0.0.1:42101/",
		"http://127.0.0.1:42101/site/opaque/?next=https://example.test", "http://127.0.0.1:42101/site/opaque/resource",
		"http://127.0.0.1:0/site/opaque/", "http://127.0.0.1:42101/site//",
	} {
		if err := browser.OpenReference(context.Background(), ReferenceReady{URL: value}); err == nil {
			t.Fatalf("Firefox accepted %q", value)
		}
	}
	if opened {
		t.Fatal("Firefox launcher received a refused URL")
	}
}

func TestFirefoxBrowserDoesNotLaunchAfterCancellation(t *testing.T) {
	browser := &FirefoxBrowser{executable: "C:\\Firefox\\firefox.exe", start: func(string, string) error {
		t.Fatal("Firefox launched after context cancellation")
		return nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := browser.OpenReference(ctx, ReferenceReady{URL: "http://127.0.0.1:42101/site/opaque/"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Firefox open error = %v", err)
	}
}
