package architecture

// Exact imports of the retained Descriptor Release signing composition after the
// transitional orchestration retired (#604 FJD stage D, ADR-0125). The signing
// cutover binds the canonical systemd.PlatformUnitTemplate and the static
// ardents-next program; no installation/descriptor, runtime/Route consumer or
// role-wide permission is introduced. The former initial admission entry point
// and detached inventory accessors retired with the orchestration.
func descriptorInitialImportAllowed(source, dependency string) bool {
	switch source {
	case "internal/release/infrastructure/signing/descriptor_release_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/release/infrastructure", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "internal/release/infrastructure/signing/descriptor_forward_release_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/release/infrastructure":
			return true
		}
	case "internal/release/infrastructure/descriptor_admission.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/domain/enrollment", "github.com/dianabuilds/ardents-network/internal/release":
			return true
		}
	}
	return false
}
