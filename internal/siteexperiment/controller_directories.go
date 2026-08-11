package siteexperiment

import (
	"os"
	"path/filepath"
)

func prepareReferenceDirectories(root string) (map[string]string, error) {
	directories := make(map[string]string)
	for _, name := range []string{"client", "service", "route", "admin", "gateway-authority", "gateway", "authority-config", "admin-config", "client-config", "evidence"} {
		directory := filepath.Join(root, name)
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, err
		}
		directories[name] = directory
	}
	for _, role := range referenceRoles {
		directory := filepath.Join(directories["evidence"], role)
		if err := os.MkdirAll(directory, 0o777); err != nil {
			return nil, err
		}
		if err := os.Chmod(directory, 0o777); err != nil {
			return nil, err
		}
	}
	for _, name := range []string{"client", "service", "route", "admin", "gateway-authority", "gateway"} {
		if err := os.Chmod(directories[name], 0o777); err != nil {
			return nil, err
		}
	}
	return directories, nil
}
