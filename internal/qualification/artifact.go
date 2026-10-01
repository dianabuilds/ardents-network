//go:build linux

package qualification

import "encoding/hex"

// Artifact reports the inventory verified before Grant delivery. These
// diagnostics cannot be supplied back as launch authority.
type Artifact struct {
	ManifestSHA256 string
	Files          map[string]string
	Cgroup         string
}

func ArtifactFrom(manifestDigest [32]byte, files map[string][32]byte, cgroup string) *Artifact {
	report := &Artifact{
		ManifestSHA256: hex.EncodeToString(manifestDigest[:]),
		Files:          make(map[string]string, len(files)),
		Cgroup:         cgroup,
	}
	for path, digest := range files {
		report.Files[path] = hex.EncodeToString(digest[:])
	}
	return report
}
