package blockedentry

const (
	finalLinuxImage = "ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960"
	finalImageHash  = "7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960"
	finalClientHash = "de581c8dd36193bb4168aee840406294af406bf8187817c10ac2bcd9464fd120"
	finalServerHash = "5fe32f8ab736ed54fc66027775761084e68f0e1ec9b5fea7c3417c6617255336"
)

var finalPublicConfigurationHashes = map[string]string{
	"configuration/topology.json":  "ad8019e0855a637ec0aa0a376aac590e87c34d4c56fc8a305e044e4e874133a6",
	"configuration/cgroups.json":   "577cdf546310d74a68f9b8efc46de5986b9be1bead0cb26949a9f271ffab4c2d",
	"configuration/network.json":   "ffba48ee8fcd1be3bfee8339a0719a601684629eafb1a262ef2ec00ec1372b5d",
	"configuration/workloads.json": "07d0f2a48adc7481ad4edb7977ddaadeed54618de3d394d072c94f3b867e23de",
	"configuration/observers.json": "30529fdc6774d484e8d4a9aef16e6ea1080ff632e3d835ed878d4404eb5b4c19",
}
