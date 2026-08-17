package modulecache

import (
	"errors"
	"os"
	"path/filepath"
)

// Generate downloads and verifies the complete graph into an empty cache,
// then atomically publishes one canonical external archive.
func Generate(options Options) (receipt Receipt, returnErr error) {
	return generate(options, "https://proxy.golang.org", "sum.golang.org")
}

func generate(options Options, proxy, sumdb string) (receipt Receipt, returnErr error) {
	workspace, target, err := resolveLocations(options)
	if err != nil {
		return Receipt{}, err
	}
	published := false
	defer func() {
		if returnErr != nil && published {
			returnErr = errors.Join(returnErr, os.Remove(target))
		}
	}()
	cache, err := os.MkdirTemp("", "ardents-stage5-gomodcache-")
	if err != nil {
		return Receipt{}, err
	}
	defer func() { returnErr = errors.Join(returnErr, os.RemoveAll(cache)) }()
	environment := moduleEnvironment(cache, proxy, sumdb)
	if _, err := boundedGo(workspace, environment, "mod", "download", "all"); err != nil {
		return Receipt{}, err
	}
	if _, err := boundedGo(workspace, environment, "mod", "verify"); err != nil {
		return Receipt{}, err
	}
	modules, err := boundedGo(workspace, environment, "list", "-m", "all")
	if err != nil {
		return Receipt{}, err
	}
	if err := pruneVolatileState(cache); err != nil {
		return Receipt{}, err
	}
	if err := os.RemoveAll(filepath.Join(cache, ".gocache")); err != nil {
		return Receipt{}, err
	}
	manifest := filepath.Join(cache, ".ardents-stage5")
	if err := os.Mkdir(manifest, 0o700); err != nil {
		return Receipt{}, err
	}
	if err := writeSourceHashes(workspace, manifest); err != nil {
		return Receipt{}, err
	}
	if err := os.WriteFile(filepath.Join(manifest, "modules.txt"), modules, 0o600); err != nil {
		return Receipt{}, err
	}
	if err := publishCanonicalArchive(cache, target); err != nil {
		return Receipt{}, err
	}
	published = true
	digest, size, err := hashFile(target)
	return Receipt{SHA256: digest, Bytes: size}, err
}
