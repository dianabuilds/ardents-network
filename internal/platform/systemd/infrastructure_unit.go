package systemd

import (
	"path"
	"strings"
)

// InfrastructureUnit holds the admitted inputs used by the signed Linux unit
// template and by the comparison against the loaded systemd service.
type InfrastructureUnit struct {
	Role           string
	Distribution   string
	Program        string
	Configuration  string
	Manifest       string
	ManifestSHA256 string
	WritableRoots  []string
}

// InfrastructureAccount is the fixed service account for an admitted
// infrastructure role.
func InfrastructureAccount(role string) string {
	if role == "source" {
		return "ardents-source"
	}
	return "ardents-node"
}

func (p InfrastructureUnit) runtimeArgs() []string {
	if p.Role == "source" {
		return []string{p.Program, "source", "serve-retained", p.Configuration}
	}
	return []string{p.Program, "node", p.Role, "--retained-config", p.Configuration}
}

func (p InfrastructureUnit) checkProgram() string {
	return path.Join(p.Distribution, "ardents-node-linux-amd64")
}

func (p InfrastructureUnit) prestartArgs() []string {
	return []string{p.checkProgram(), "platform", "check", p.Manifest, p.ManifestSHA256}
}

// Render binds the admitted role and paths to the canonical signed template.
func (p InfrastructureUnit) Render() []byte {
	runtime, prestart := p.runtimeArgs(), p.prestartArgs()
	r := strings.NewReplacer(
		"@ROLE@", p.Role, "@USER@", InfrastructureAccount(p.Role),
		"@CHECK_PROGRAM@", prestart[0], "@MANIFEST@", prestart[3], "@SHA256@", prestart[4],
		"@PROGRAM@", p.Program, "@RUNTIME@", strings.Join(runtime[1:], " "), "@WRITABLE@", strings.Join(p.WritableRoots, " "),
	)
	return []byte(r.Replace(PlatformUnitTemplate))
}
