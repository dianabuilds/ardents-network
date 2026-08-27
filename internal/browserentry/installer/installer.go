package installer

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/browserentry"
)

const (
	maximumExtensionSize       = 64 << 20
	maximumExtensionEntryBytes = 64 << 10
)

// Result identifies the two local bytes participating in a completed native
// manifest installation. The user still installs the Mozilla-signed XPI in
// Firefox explicitly; this package does not launch or configure Firefox.
type Result struct {
	NativeManifestPath string
	ExtensionPath      string
}

type nativeManifest struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Path              string   `json:"path"`
	Type              string   `json:"type"`
	AllowedExtensions []string `json:"allowed_extensions"`
}

type location struct {
	path       string
	register   func(string) error
	unregister func(string) error
}

// Install verifies that extensionPath is an XPI for the fixed alpha extension
// with Mozilla's current COSE signing surface, then installs or replaces only
// the caller's per-user native-manifest registration for the supplied absolute
// host binary. Firefox remains the cryptographic signature verifier when the
// participant explicitly installs the XPI; caller-owned release verification
// authenticates the exact host and XPI bytes before this function is called.
func Install(hostPath, extensionPath string) (Result, error) {
	installed, err := installAt(hostPath, extensionPath, defaultLocation())
	if err != nil {
		return Result{}, err
	}
	return installed, nil
}

// Remove withdraws only the fixed alpha native-manifest registration. It does
// not remove the Firefox extension, Endpoint state, Authority, corpus floor,
// or any other Firefox configuration.
func Remove() error {
	return removeAt(defaultLocation())
}

func installAt(hostPath, extensionPath string, target location) (Result, error) {
	host, err := regularAbsoluteFile(hostPath)
	if err != nil {
		return Result{}, errors.New("browser Entry host path is invalid")
	}
	extension, err := regularAbsoluteFile(extensionPath)
	if err != nil || !validExtension(extension) {
		return Result{}, errors.New("browser Entry extension is invalid")
	}
	if target.path == "" || target.register == nil || target.unregister == nil || !filepath.IsAbs(target.path) {
		return Result{}, errors.New("browser Entry native manifest location is invalid")
	}
	body, err := json.Marshal(nativeManifest{Name: browserentry.NativeHostName, Description: "Ardents Alpha Browser Entry native host",
		Path: host, Type: "stdio", AllowedExtensions: []string{browserentry.FirefoxExtensionID}})
	if err != nil {
		return Result{}, err
	}
	if err := replaceOwnedManifest(target.path, body); err != nil {
		return Result{}, err
	}
	if err := target.register(target.path); err != nil {
		return Result{}, err
	}
	return Result{NativeManifestPath: target.path, ExtensionPath: extension}, nil
}

func removeAt(target location) error {
	if target.path == "" || target.unregister == nil || !filepath.IsAbs(target.path) {
		return errors.New("browser Entry native manifest location is invalid")
	}
	info, err := os.Lstat(target.path)
	if os.IsNotExist(err) {
		return target.unregister(target.path)
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("browser Entry native manifest is not owned")
	}
	raw, err := os.ReadFile(target.path)
	if err != nil || !ownedManifest(raw) {
		return errors.New("browser Entry native manifest is not owned")
	}
	if err := target.unregister(target.path); err != nil {
		return err
	}
	return os.Remove(target.path)
}

func regularAbsoluteFile(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("not absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 {
		return "", errors.New("not a regular file")
	}
	return path, nil
}

func validExtension(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.Size() > maximumExtensionSize {
		return false
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer archive.Close()
	var manifest []byte
	metadata := make(map[string]bool, 2)
	for _, file := range archive.File {
		switch file.Name {
		case "manifest.json":
			if manifest != nil {
				return false
			}
			contents, readErr := boundedArchiveEntry(file)
			if readErr != nil {
				return false
			}
			manifest = contents
		case "META-INF/cose.manifest", "META-INF/cose.sig":
			if metadata[file.Name] {
				return false
			}
			if _, readErr := boundedArchiveEntry(file); readErr != nil {
				return false
			}
			metadata[file.Name] = true
		}
	}
	if manifest == nil || !metadata["META-INF/cose.manifest"] || !metadata["META-INF/cose.sig"] {
		return false
	}
	var value struct {
		BrowserSpecificSettings struct {
			Gecko struct {
				ID string `json:"id"`
			} `json:"gecko"`
		} `json:"browser_specific_settings"`
	}
	return json.Unmarshal(manifest, &value) == nil && value.BrowserSpecificSettings.Gecko.ID == browserentry.FirefoxExtensionID
}

func boundedArchiveEntry(file *zip.File) ([]byte, error) {
	if file.UncompressedSize64 == 0 || file.UncompressedSize64 > maximumExtensionEntryBytes {
		return nil, errors.New("browser entry XPI entry has invalid bounds")
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	contents, err := io.ReadAll(io.LimitReader(reader, maximumExtensionEntryBytes+1))
	if err != nil || len(contents) == 0 || len(contents) > maximumExtensionEntryBytes {
		return nil, errors.New("browser entry XPI entry cannot be read")
	}
	return contents, nil
}

func replaceOwnedManifest(path string, body []byte) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("browser Entry native manifest parent is invalid")
	}
	priorInfo, statErr := os.Lstat(path)
	if statErr == nil {
		if !priorInfo.Mode().IsRegular() || priorInfo.Mode()&os.ModeSymlink != 0 {
			return errors.New("browser Entry native manifest is not owned")
		}
		prior, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if !ownedManifest(prior) {
			return errors.New("browser Entry native manifest is not owned")
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	temporary, err := os.CreateTemp(parent, ".ardents-native-manifest-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func ownedManifest(raw []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest nativeManifest
	if err := decoder.Decode(&manifest); err != nil || manifest.Name != browserentry.NativeHostName ||
		manifest.Description != "Ardents Alpha Browser Entry native host" || !filepath.IsAbs(manifest.Path) ||
		manifest.Type != "stdio" || len(manifest.AllowedExtensions) != 1 || manifest.AllowedExtensions[0] != browserentry.FirefoxExtensionID {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}
