//go:build linux

package endpoint

import (
	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
	"github.com/dianabuilds/ardents-network/internal/qualification"
)

func (lifetime *workerLifetime) qualificationArtifact() *qualification.Artifact {
	if lifetime.artifact == nil || lifetime.artifact.Inventory() != worker.Stream {
		return nil
	}
	return qualification.ArtifactFrom(lifetime.artifact.ManifestDigest(), lifetime.artifact.FileDigests(), lifetime.cgroup)
}
