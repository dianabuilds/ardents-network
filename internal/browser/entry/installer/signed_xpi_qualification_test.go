//go:build browser_signed_xpi

package installer

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/browser/entry"
)

const alphaBrowserEntrySignedXPIHash = "d88e8ecba84cda82a7b2354d1f445e19b9d092f3f3d068868d1173ef29eaa2a2"

func TestMozillaSignedAlphaBrowserEntryXPI(t *testing.T) {
	path := os.Getenv("ARDENTS_BROWSER_SIGNED_XPI")
	if path == "" {
		t.Fatal("ARDENTS_BROWSER_SIGNED_XPI must identify the Mozilla-signed XPI")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("Mozilla-signed XPI is not a regular file: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != alphaBrowserEntrySignedXPIHash {
		t.Fatalf("Mozilla-signed XPI digest = %x, want %s", digest, alphaBrowserEntrySignedXPIHash)
	}
	if !validExtension(path) {
		t.Fatal("Mozilla-signed XPI does not have the accepted fixed-ID COSE surface")
	}
	manifest, err := signedXPIManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ManifestVersion != 3 || manifest.Version != "0.1.0" || manifest.BrowserSpecificSettings.Gecko.ID != browserentry.FirefoxExtensionID ||
		!sameStrings(manifest.Permissions, []string{"proxy", "nativeMessaging", "webRequest", "webRequestBlocking"}) ||
		!sameStrings(manifest.HostPermissions, []string{"http://*.ard/*", "https://*.ard/*"}) {
		t.Fatalf("Mozilla-signed XPI manifest is outside the reviewed alpha boundary: %+v", manifest)
	}
}

type alphaBrowserEntryManifest struct {
	ManifestVersion         int      `json:"manifest_version"`
	Version                 string   `json:"version"`
	Permissions             []string `json:"permissions"`
	HostPermissions         []string `json:"host_permissions"`
	BrowserSpecificSettings struct {
		Gecko struct {
			ID string `json:"id"`
		} `json:"gecko"`
	} `json:"browser_specific_settings"`
}

func signedXPIManifest(path string) (alphaBrowserEntryManifest, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return alphaBrowserEntryManifest{}, err
	}
	defer archive.Close()
	for _, entry := range archive.File {
		if entry.Name != "manifest.json" {
			continue
		}
		reader, openErr := entry.Open()
		if openErr != nil {
			return alphaBrowserEntryManifest{}, openErr
		}
		raw, readErr := io.ReadAll(io.LimitReader(reader, maximumExtensionEntryBytes+1))
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil || len(raw) == 0 || len(raw) > maximumExtensionEntryBytes {
			return alphaBrowserEntryManifest{}, errors.New("signed XPI manifest cannot be read")
		}
		var manifest alphaBrowserEntryManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return alphaBrowserEntryManifest{}, err
		}
		return manifest, nil
	}
	return alphaBrowserEntryManifest{}, errors.New("signed XPI manifest is absent")
}

func sameStrings(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}
