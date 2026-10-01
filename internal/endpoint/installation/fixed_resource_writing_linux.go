//go:build linux

package installation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func installInitialFixedResources(ctx context.Context, root string, selected selection) (returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 {
		return errors.New("fixed resource installation requires root and context")
	}
	selectedBytes, err := canonicalJSON(selected)
	if err != nil {
		return err
	}
	checked, err := readLocalBinding(root, func(path string, maximum int64) ([]byte, error) {
		if path == filepath.Join(root, "selection.json") {
			return selectedBytes, nil
		}
		return readInstalledFile(path, maximum)
	})
	if err != nil {
		return err
	}
	if err := observeAccountAndRoots(checked, false); err != nil {
		return err
	}
	if err := refuseLoadedInstallationUnits(ctx); err != nil {
		return err
	}
	for path := range fixedResources() {
		if err := requireAbsentManagedPath(path); err != nil {
			return err
		}
	}
	for _, path := range []string{"/usr/lib/ardents/text-worker-root", "/etc/ardents/text-worker-artifact.json", "/run/ardents-text"} {
		if err := requireAbsentManagedPath(path); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	lease, err := acquireInstallationLease(root)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, lease.Close()) }()
	journal := filepath.Join(root, "journals", selected.GenerationDigest)
	if err := requireStagedJournal(journal, selected); err != nil {
		return err
	}
	if err := appendGenerationRecord(journal, "0003.json", selected, "installing-fixed-resources", nil); err != nil {
		return err
	}
	defer func() {
		if returnedErr != nil {
			returnedErr = errors.Join(returnedErr, appendGenerationRecord(journal, "fixed-resource-failure.json", selected, "fixed-resources-failed", returnedErr))
		}
	}()
	workerRoot := "/usr/lib/ardents/text-worker-root"
	if err := createRootDirectory(workerRoot, 0700, 0); err != nil {
		return err
	}
	for _, name := range []string{"dev", "proc", "sys", "run", "tmp", "etc", "root", "usr", "var", "var/tmp"} {
		if err := createRootDirectory(filepath.Join(workerRoot, name), 0555, 0); err != nil {
			return err
		}
	}
	paths := make([]string, 0, len(fixedResources()))
	for path := range fixedResources() {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	digests := map[string]string{}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := fixedResources()[path]
		if name == "ardents-endpoint.service" {
			name = "endpoint-unit.service"
		}
		body := checked.files[name]
		if len(body) == 0 {
			return errors.New("fixed installation resource is absent")
		}
		if err := ensureRootDirectory(filepath.Dir(path)); err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if path == filepath.Join(workerRoot, "ardents-text") {
			mode = 0555
		}
		if err := createInstallationFile(ctx, path, body, mode, 0, journal, selected); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(path)); err != nil {
			return err
		}
		if name != "endpoint-unit.service" && name != "ardents-text.conf" {
			digests[path] = digestHex(body)
		}
	}
	if err := os.Chmod(workerRoot, 0555); err != nil {
		return err
	}
	if err := syncDirectory(workerRoot); err != nil {
		return err
	}
	manifest, err := canonicalJSON(struct {
		Schema string            `json:"schema"`
		Files  map[string]string `json:"files"`
	}{"ardents-text-worker-artifact-v1", digests})
	if err != nil {
		return err
	}
	if err := ensureRootDirectory("/etc/ardents"); err != nil {
		return err
	}
	if err := createInstallationFile(ctx, "/etc/ardents/text-worker-artifact.json", manifest, 0644, 0, journal, selected); err != nil {
		return err
	}
	if err := syncDirectory("/etc/ardents"); err != nil {
		return err
	}
	if err := createRootDirectory("/run/ardents-text", 0710, checked.binding.GID); err != nil {
		return err
	}
	if err := observeFixedResources(checked); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return appendGenerationRecord(journal, "0004.json", selected, "fixed-resources-installed", nil)
}

func requireStagedJournal(directory string, selected selection) error {
	return requireJournalPhases(directory, selected, []string{"writing-generation", "generation-staged"})
}

func requireJournalPhases(directory string, selected selection, phases []string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return errors.New("installation staging journal is ambiguous")
	}
	count := len(entries)
	for _, entry := range entries {
		if entry.Name() == "creations" {
			if err := requirePrivateJournalDirectory(filepath.Join(directory, entry.Name())); err != nil {
				return err
			}
			count--
		}
		if entry.Name() == "generation-directory.json" {
			if err := verifyGenerationDirectory(directory, filepath.Join(filepath.Dir(filepath.Dir(directory)), "generations", selected.GenerationDigest), selected); err != nil {
				return err
			}
			count--
		}
	}
	if count != len(phases) {
		return errors.New("installation staging journal is ambiguous")
	}
	for index, phase := range phases {
		body, err := readInstalledFile(filepath.Join(directory, fmt.Sprintf("%04d.json", index+1)), 64<<10)
		if err != nil {
			return err
		}
		var record transitionRecord
		if err := decodeCanonical(body, 64<<10, &record); err != nil {
			return err
		}
		if record.Schema != "ardents-endpoint-installation-transition-v1" || record.GenerationDigest != selected.GenerationDigest || record.BindingDigest != selected.BindingDigest || record.Phase != phase || record.OriginalError != "" {
			return errors.New("installation staging journal does not bind the candidate")
		}
	}
	return nil
}

func ensureRootDirectory(path string) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return createRootDirectory(path, 0755, 0)
	}
	return checkRootAncestors(path, false)
}

func createRootDirectory(path string, mode os.FileMode, gid uint32) error {
	if err := requireAbsentManagedPath(path); err != nil {
		return err
	}
	if err := ensureRootDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0700); err != nil {
		return err
	}
	if err := os.Chown(path, 0, int(gid)); err != nil {
		return err
	}
	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	if err := syncDirectory(path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
