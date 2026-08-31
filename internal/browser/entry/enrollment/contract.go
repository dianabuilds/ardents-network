package enrollment

const (
	inputSchema      = "ardents-alpha-enrollment-input-v1"
	descriptorSchema = "ardents-closed-alpha-enrollment-v4"
	manifestName     = "SHA256SUMS"
	descriptorName   = "RELEASE"
	maximumFiles     = 32
	maximumFileLen   = 64 << 20
	maximumInputSize = 16 << 10
)

// Pin is the independently delivered fact that authenticates one exact v4
// Browser companion inventory.
type Pin struct {
	Cohort, Release, Platform string
	ManifestSHA256            string
}

// Request identifies one local v4 bundle and its enrolled Endpoint artifact.
type Request struct {
	BundleRoot, ExecutablePath       string
	Pin                              Pin
	Environment, Network, TargetPath string
}

// Verified contains only the Application-owned companions consumed by the
// Browser Entry installer. Network metadata and implementation types do not
// cross this boundary.
type Verified struct {
	BrowserAdapterArtifactName string
	BrowserAdapterArtifact     []byte
	BrowserEntryArtifactName   string
	BrowserEntryArtifact       []byte
	BrowserEntryExtensionName  string
	BrowserEntryExtension      []byte
}
