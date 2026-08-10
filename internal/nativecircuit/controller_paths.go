package nativecircuit

import "path/filepath"

func initialNativeReadyPaths(fixture nativeFixture) []string {
	paths := make([]string, 0, len(nativeNodeRoles)+len(fixture.toolEvidence))
	for _, role := range nativeNodeRoles {
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

func nativeComposeServices() []string {
	services := append([]string(nil), nativeApplicationRoles...)
	for _, role := range nativeApplicationRoles {
		services = append(services, "shape-"+role)
	}
	for _, role := range []string{"user", "user-entry", "user-interior", "rendezvous", "service-interior", "data-service-entry", "introduction-forwarder", "introduction-node", "introduction-interior", "introduction-entry"} {
		services = append(services, "capture-"+role)
	}
	return services
}
