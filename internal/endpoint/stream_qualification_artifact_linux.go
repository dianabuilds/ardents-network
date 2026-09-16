//go:build linux

package endpoint

import "encoding/hex"

// StreamQualificationArtifact reports the inventory verified before Grant
// delivery. These diagnostics cannot be supplied back as launch authority.
type StreamQualificationArtifact struct {
	ManifestSHA256 string
	Files          map[string]string
	Cgroup         string
}

func (worker *textWorkerLifetime) qualificationArtifact() *StreamQualificationArtifact {
	if worker.artifact == nil || worker.artifact.inventory != streamInventory {
		return nil
	}
	report := &StreamQualificationArtifact{
		ManifestSHA256: hex.EncodeToString(worker.artifact.manifestDigest[:]),
		Files:          make(map[string]string, len(worker.artifact.files)),
		Cgroup:         worker.cgroup,
	}
	for path, digest := range worker.artifact.files {
		report.Files[path] = hex.EncodeToString(digest[:])
	}
	return report
}
