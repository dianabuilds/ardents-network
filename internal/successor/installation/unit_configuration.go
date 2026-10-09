package installation

import (
	"errors"
	endpointunit "github.com/dianabuilds/ardents-network/internal/successor/installation/unit"
	"path"
	"sort"
	"strings"
	"unicode"
)

// Template checks precede writable-root admission, retaining the original first
// refusal. Installation owns path selection; unit owns exact byte rendering.
func renderEndpointUnit(template []byte, request installationRequest, directory string) ([]byte, error) {
	prepared, err := endpointunit.PrepareEndpointTemplate(template, request.InstallationRoot, directory)
	if err != nil {
		return nil, err
	}
	expected, err := expectedUnit(request)
	if err != nil {
		return nil, err
	}
	return prepared.Render(expected.WritePaths)
}

// Request write-root admission stays with Installation. The fixed unit Module
// receives detached expected facts, never the private request or live custody.
func expectedUnit(request installationRequest) (endpointunit.Configuration, error) {
	paths, err := writableDirectories(request)
	return endpointunit.Configuration{InstallationRoot: request.InstallationRoot, WritePaths: paths}, err
}

// These are the actual write allowances rendered into the fixed unit. Include
// socket/permission parent directories, rather than checking only their leaves:
// a writable parent must not enclose immutable artifacts or retained trust.
func writableDirectories(request installationRequest) ([]string, error) {
	paths := mutableRoots(request.Headless)
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
