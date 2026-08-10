package nativecircuit

import "path/filepath"

func initialNativeReadyPaths(fixture nativeFixture, topology nativeTopology) []string {
	paths := make([]string, 0, len(topology.nodeRoles)+len(fixture.toolEvidence))
	for _, role := range topology.nodeRoles {
		paths = append(paths, filepath.Join(fixture.roleEvidence[role], "ready.json"))
	}
	for _, directory := range fixture.toolEvidence {
		paths = append(paths, filepath.Join(directory, "ready.json"))
	}
	return paths
}

func nativeCaptureReadyPaths(fixture nativeFixture) []string {
	var paths []string
	for name, directory := range fixture.toolEvidence {
		if len(name) > len("capture-") && name[:len("capture-")] == "capture-" {
			paths = append(paths, filepath.Join(directory, "capture-ready.json"))
		}
	}
	return paths
}
