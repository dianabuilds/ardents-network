//go:build linux

package endpoint

import "github.com/dianabuilds/ardents-network/internal/qualification"

func (worker *textWorkerLifetime) qualificationArtifact() *qualification.Artifact {
	if worker.artifact == nil || worker.artifact.inventory != streamInventory {
		return nil
	}
	files := make(map[string][32]byte, len(worker.artifact.files))
	for path, digest := range worker.artifact.files {
		files[path] = digest
	}
	return qualification.ArtifactFrom(worker.artifact.manifestDigest, files, worker.cgroup)
}
