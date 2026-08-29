package contributor

import "path/filepath"

type hostPaths struct {
	root, programRoot, programCurrent, privateRoot, configRoot, configCurrent string
	diagnostics, lifecycle, record, unit                                      string
	installing                                                                string
}

func newHostPaths(root string) hostPaths {
	programRoot := filepath.Join(root, "usr", "lib", "ardents-contributor")
	privateRoot := filepath.Join(root, "var", "lib", "private", "ardents-contributor")
	configRoot := filepath.Join(privateRoot, "config")
	diagnostics := filepath.Join(privateRoot, "diagnostics")
	return hostPaths{root: root, programRoot: programRoot, programCurrent: filepath.Join(programRoot, "current"),
		privateRoot: privateRoot, configRoot: configRoot, configCurrent: filepath.Join(configRoot, "current"),
		diagnostics: diagnostics, lifecycle: filepath.Join(diagnostics, "lifecycle.json"),
		record:     filepath.Join(privateRoot, "installation.json"),
		installing: filepath.Join(root, "var", "lib", "private", "ardents-contributor-installing.json"),
		unit:       filepath.Join(root, "etc", "systemd", "system", "ardents-rendezvous-contributor.service")}
}

func installedPath(name string) string {
	switch name {
	case "ardents-node":
		return "/usr/lib/ardents-contributor/current/ardents-node"
	case "node.json":
		return "/var/lib/private/ardents-contributor/config/current/node.json"
	default:
		return "/var/lib/private/ardents-contributor/config/current/" + name
	}
}
