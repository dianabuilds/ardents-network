//go:build windows

package installer

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"

	"github.com/dianabuilds/ardents-network/internal/browserentry"
)

const nativeManifestRegistryPath = "Software\\Mozilla\\NativeMessagingHosts\\" + browserentry.NativeHostName

func defaultLocation() location {
	root, err := os.UserConfigDir()
	if err != nil {
		return location{}
	}
	path := filepath.Join(root, "Ardents", "browser-entry", "native-host", browserentry.NativeHostName+".json")
	return location{path: path, register: registerCurrentUserManifest, unregister: unregisterCurrentUserManifest}
}

func registerCurrentUserManifest(path string) error {
	key, existed, err := registry.CreateKey(registry.CURRENT_USER, nativeManifestRegistryPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	if existed {
		defer key.Close()
		prior, _, readErr := key.GetStringValue("")
		if readErr != nil || prior != path {
			return errors.New("browser Entry native manifest registration is not owned")
		}
		return key.SetStringValue("", path)
	}
	if err := key.SetStringValue("", path); err != nil {
		closeErr := key.Close()
		if closeErr == nil {
			_ = registry.DeleteKey(registry.CURRENT_USER, nativeManifestRegistryPath)
		}
		return err
	}
	return key.Close()
}

func unregisterCurrentUserManifest(path string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, nativeManifestRegistryPath, registry.QUERY_VALUE)
	if err == registry.ErrNotExist {
		return nil
	}
	if err != nil {
		return err
	}
	value, _, valueErr := key.GetStringValue("")
	closeErr := key.Close()
	if valueErr != nil {
		return valueErr
	}
	if closeErr != nil {
		return closeErr
	}
	if value != path {
		return errors.New("browser Entry native manifest registration is not owned")
	}
	return registry.DeleteKey(registry.CURRENT_USER, nativeManifestRegistryPath)
}
