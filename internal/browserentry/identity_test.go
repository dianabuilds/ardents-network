package browserentry

import "testing"

func TestHostArtifactNameUsesTheNativeWindowsExecutableSuffix(t *testing.T) {
	if got := HostArtifactName("windows-amd64"); got != "ardents-browser-entry-windows-amd64.exe" {
		t.Fatalf("Windows Browser Entry artifact name = %q", got)
	}
	if got := HostArtifactName("linux-amd64"); got != "ardents-browser-entry-linux-amd64" {
		t.Fatalf("Linux Browser Entry artifact name = %q", got)
	}
}
