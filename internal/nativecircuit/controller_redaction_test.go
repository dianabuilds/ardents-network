package nativecircuit

import (
	"strings"
	"testing"
)

func TestNativeFailureEvidenceRedactsHostPathsAndAddresses(t *testing.T) {
	t.Parallel()
	layout := nativeRunLayout{
		repositoryRoot: `C:\source\ardents`, runDirectory: `C:\temp\session\run`, evidenceDir: `C:\temp\session\evidence`,
	}
	message := `mount C:\temp\session\run from C:\source\ardents failed via 192.0.2.8; see C:/temp/session/evidence`
	redacted := sanitizeNativeFailure(layout, message)
	for _, forbidden := range []string{`C:\temp`, `C:/temp`, `C:\source`, "192.0.2.8"} {
		if strings.Contains(redacted, forbidden) {
			t.Fatalf("retained native failure leaked %q: %s", forbidden, redacted)
		}
	}
}
