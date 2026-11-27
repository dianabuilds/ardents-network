package infrastructureinventory

const (
	// Name is the canonical filename of the inventory document.
	Name = "infrastructure-enrollment.json"
	// MaximumSize bounds the canonical document and each role descriptor.
	MaximumSize = 16 << 10
	// ProgramName is the enrolled Linux amd64 executable filename.
	ProgramName = "ardents-node-linux-amd64"

	maximumArtifactSize = 64 << 20
	maximumMetadataSize = 1 << 20
	fileCount           = 7
)

// File identifies one required companion and its exact byte bound.
type File struct {
	Name        string
	MaximumSize int64
}

// Files returns the fixed companions in lexical order for a supported invocation.
func Files(invocation string) []File {
	switch invocation {
	case "source", "forwarding", "descriptor", "join", "introduction", "issuer":
	default:
		return nil
	}
	first := File{"ardents-" + invocation + ".service", maximumArtifactSize}
	second := File{ProgramName, maximumArtifactSize}
	if first.Name > second.Name {
		first, second = second, first
	}
	return []File{
		first, second,
		{"infrastructure-" + invocation + ".json", MaximumSize},
		{"root.json", maximumMetadataSize},
		{"snapshot.json", maximumMetadataSize},
		{"targets.json", maximumMetadataSize},
		{"timestamp.json", maximumMetadataSize},
	}
}
