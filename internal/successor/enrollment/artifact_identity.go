package enrollment

import "strings"

// ExecutableArtifactName returns the enrollment-owned command identity on a
// declared platform. It formats an inventory name only; it validates neither
// platform support nor artifact bytes and grants no enrollment provenance.
func ExecutableArtifactName(command, platform string) string {
	name := command + "-" + platform
	if strings.HasPrefix(platform, "windows-") {
		name += ".exe"
	}
	return name
}
