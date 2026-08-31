package browserentry

const (
	// NativeHostName is the fixed Firefox native-messaging host identity owned
	// by the alpha Browser Entry profile.
	NativeHostName = "org.ardents.alpha_browser_entry"
	// FirefoxExtensionID is the one fixed extension allowed to invoke the
	// alpha Browser Entry native host.
	FirefoxExtensionID = "alpha-browser-entry@ardents.network"
	// ExtensionArtifactName is the fixed self-distributed, Mozilla-signed XPI
	// filename in an enrolled Browser Entry release bundle.
	ExtensionArtifactName = "ardents-alpha-browser-entry.xpi"
)

// HostArtifactName returns the fixed platform-specific native-host filename
// in an enrolled Browser Entry release bundle.
func HostArtifactName(platform string) string {
	name := "ardents-browser-entry-" + platform
	if len(platform) > len("windows-") && platform[:len("windows-")] == "windows-" {
		return name + ".exe"
	}
	return name
}
