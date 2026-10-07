package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

// Initial fixed publication retains stopped manager observations and the
// original preparation. This private operation grants no platform admission.
func (owned *initialPreparation) installFixedResources(ctx context.Context, request Request) error {
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	if err := observeAbsentInstallationUnits(ctx); err != nil {
		return err
	}
	for filename := range fixedResourceNames() {
		if err := requireAbsentManagedPath(filename); err != nil {
			return err
		}
	}
	for _, filename := range []string{"/usr/lib/ardents/text-worker-root", "/etc/ardents/text-worker-artifact.json", "/run/ardents-text"} {
		if err := requireAbsentManagedPath(filename); err != nil {
			return err
		}
	}
	stage := owned.stage
	if err := stage.fixedPhase(ctx, "0003.json", "installing-fixed-resources"); err != nil {
		return err
	}
	workerRoot := "/usr/lib/ardents/text-worker-root"
	if err := stage.birthFixedDirectory(ctx, workerRoot, 0700, 0); err != nil {
		return err
	}
	for _, name := range []string{"dev", "proc", "sys", "run", "tmp", "etc", "root", "usr", "var", "var/tmp"} {
		if err := stage.birthFixedDirectory(ctx, filepath.Join(workerRoot, name), 0555, 0); err != nil {
			return err
		}
	}
	resources := fixedResourceNames()
	paths := make([]string, 0, len(resources))
	for filename := range resources {
		paths = append(paths, filename)
	}
	sort.Strings(paths)
	digests := make(map[string]string)
	for _, filename := range paths {
		name := resources[filename]
		if name == "ardents-endpoint.service" {
			name = "endpoint-unit.service"
		}
		body := stage.generation.files[name].body
		if len(body) == 0 {
			return ErrBinding
		}
		if err := stage.ensureRootParent(ctx, filepath.Dir(filename)); err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if filename == filepath.Join(workerRoot, "ardents-text") {
			mode = 0555
		}
		if err := stage.createFixedFile(ctx, filename, body, mode, 0); err != nil {
			return err
		}
		if name != "endpoint-unit.service" && name != "ardents-text.conf" {
			digests[filename] = digestHex(body)
		}
	}
	if err := stage.changeFixedDirectoryMode(ctx, workerRoot, 0555); err != nil {
		return err
	}
	manifest, err := canonicalJSON(struct {
		Schema string            `json:"schema"`
		Files  map[string]string `json:"files"`
	}{"ardents-text-worker-artifact-v1", digests})
	if err != nil {
		return err
	}
	if err := stage.ensureRootParent(ctx, "/etc/ardents"); err != nil {
		return err
	}
	if err := stage.createFixedFile(ctx, "/etc/ardents/text-worker-artifact.json", manifest, 0644, 0); err != nil {
		return err
	}
	if err := stage.birthFixedDirectory(ctx, "/run/ardents-text", 0710, owned.prepared.gid); err != nil {
		return err
	}
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	return stage.fixedPhase(ctx, "0004.json", "fixed-resources-installed")
}

func (stage *generationStage) fixedPhase(ctx context.Context, name, phase string) error {
	if name == "0003.json" && phase == "installing-fixed-resources" {
		if _, complete := stage.journal.files["0002.json"]; !complete {
			return ErrBinding
		}
	} else if name == "0004.json" && phase == "fixed-resources-installed" {
		if _, intent := stage.journal.files["0003.json"]; !intent {
			return ErrBinding
		}
	} else if name == "0005.json" && phase == "publishing-selection" {
		if _, complete := stage.journal.files["0004.json"]; !complete {
			return ErrBinding
		}
	} else if name == "0006.json" && phase == "reloading-manager" {
		if _, intent := stage.journal.files["0005.json"]; !intent {
			return ErrBinding
		}
		if _, selected := stage.fixed[filepath.Join(stage.lease.path, "selection.json")]; !selected {
			return ErrBinding
		}
	} else if name == "0007.json" && phase == "installed-stopped" {
		if _, intent := stage.journal.files["0006.json"]; !intent {
			return ErrBinding
		}
	} else {
		return ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	body, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: phase})
	if err != nil {
		return err
	}
	return stage.journal.write(ctx, name, body, 0600, 0)
}

func (stage *generationStage) ensureRootParent(ctx context.Context, directory string) error {
	if _, err := os.Lstat(directory); os.IsNotExist(err) {
		return stage.birthFixedDirectory(ctx, directory, 0755, 0)
	}
	_, err := rootDirectoryAncestors(directory)
	return err
}

func (stage *generationStage) birthFixedDirectory(ctx context.Context, directory string, mode os.FileMode, gid uint32) (returnedErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if err := requireAbsentManagedPath(directory); err != nil {
		return err
	}
	if err := stage.ensureRootParent(ctx, filepath.Dir(directory)); err != nil {
		return err
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(directory, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	birth, err := file.Stat()
	pathBirth, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !privateJournalDirectory(birth) || !os.SameFile(birth, pathBirth) {
		return errors.Join(ErrBinding, err, pathErr)
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	info, err := file.Stat()
	pathInfo, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !os.SameFile(birth, info) || !sameStagingDirectory(info, pathInfo) || info.Mode() != os.ModeDir|mode {
		return errors.Join(ErrBinding, err, pathErr)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != gid {
		return ErrBinding
	}
	if stage.fixedDirectories == nil {
		stage.fixedDirectories = make(map[string]os.FileInfo)
	}
	stage.fixedDirectories[directory] = info
	return errors.Join(syncDirectDirectory(filepath.Dir(directory)), ctx.Err())
}

func (stage *generationStage) changeFixedDirectoryMode(ctx context.Context, directory string, mode os.FileMode) (returnedErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	original := stage.fixedDirectories[directory]
	file, err := os.OpenFile(directory, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	before, err := file.Stat()
	if err != nil || !sameStagingDirectory(original, before) {
		return errors.Join(ErrBinding, err)
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	info, err := file.Stat()
	pathInfo, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !os.SameFile(original, info) || !sameStagingDirectory(info, pathInfo) || info.Mode() != os.ModeDir|mode {
		return errors.Join(ErrBinding, err, pathErr)
	}
	stage.fixedDirectories[directory] = info
	// The owned parent changed access on this exact inode. Its already pinned
	// children keep their original file identities and bytes, but must observe
	// the new parent mode rather than treating our promotion as substitution.
	for filename, observation := range stage.fixed {
		if filepath.Dir(filename) == directory {
			if !sameStagingDirectory(original, observation.parent) {
				return ErrBinding
			}
			observation.parent = info
			stage.fixed[filename] = observation
		}
	}
	return errors.Join(stage.observe(), ctx.Err())
}
