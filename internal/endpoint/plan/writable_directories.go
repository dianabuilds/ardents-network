package plan

import (
	"errors"
	"path"
	"sort"
	"strings"
	"unicode"
)

// WritableDirectories derives the actual write allowances for the fixed unit. Include
// socket/permission parent directories, rather than checking only their leaves:
// a writable parent must not enclose immutable artifacts or retained trust.
func WritableDirectories(request Declaration) ([]string, error) {
	paths := MutableRoots(request.Headless)
	for _, file := range []string{request.Headless.ApplicationSocket, request.Headless.ReaderPermission.RequestPath, request.Headless.ReaderPermission.ResponsePath} {
		paths = append(paths, path.Dir(file))
	}
	if request.Headless.Role == "" {
		for _, file := range []string{request.Headless.AdministrationSocket, request.Headless.PublisherPermission.RequestPath, request.Headless.PublisherPermission.ResponsePath} {
			paths = append(paths, path.Dir(file))
		}
	}
	for _, writable := range paths {
		if !unitPath(writable) || writable == "/" {
			return nil, errors.New("installation writable directory is invalid")
		}
		for _, immutable := range []string{request.BundleRoot, request.InstallationRoot, request.ReleaseFloorRoot} {
			if pathsOverlap(writable, immutable) {
				return nil, errors.New("installation write allowance overlaps immutable or Release roots")
			}
		}
	}
	sort.Strings(paths)
	distinct := make([]string, 0, len(paths))
	for _, writable := range paths {
		if len(distinct) == 0 || writable != distinct[len(distinct)-1] {
			distinct = append(distinct, writable)
		}
	}
	return distinct, nil
}

func unitPath(value string) bool {
	return canonicalPath(value) && strings.IndexFunc(value, unicode.IsControl) == -1
}
