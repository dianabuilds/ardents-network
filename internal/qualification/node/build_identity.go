package node

import (
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"path/filepath"
	runtimedebug "runtime/debug"
)

type nodeBuildReceipt struct {
	Path         string            `json:"path"`
	GoVersion    string            `json:"go_version"`
	Module       nodeBuildModule   `json:"module"`
	Dependencies []nodeBuildModule `json:"dependencies"`
	Settings     map[string]string `json:"settings"`
}

type nodeBuildModule struct {
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
	Sum     string `json:"sum,omitempty"`
	Replace string `json:"replace,omitempty"`
}

func readCandidateBuildIdentity(paths []string) ([]byte, error) {
	if len(paths) == 0 || len(paths) > 8 {
		return nil, errors.New("candidate build identity path count is invalid")
	}
	receipts := make([]nodeBuildReceipt, 0, len(paths))
	for _, path := range paths {
		if len(path) == 0 || len(path) > 4096 || !filepath.IsAbs(path) {
			return nil, errors.New("candidate build identity path is invalid")
		}
		info, err := buildinfo.ReadFile(path)
		if err != nil {
			return nil, err
		}
		receipt := nodeBuildReceipt{Path: path, GoVersion: info.GoVersion, Module: nodeModuleReceipt(&info.Main),
			Dependencies: make([]nodeBuildModule, 0, len(info.Deps)), Settings: make(map[string]string, len(info.Settings))}
		for _, dependency := range info.Deps {
			receipt.Dependencies = append(receipt.Dependencies, nodeModuleReceipt(dependency))
		}
		for _, setting := range info.Settings {
			receipt.Settings[setting.Key] = setting.Value
		}
		receipts = append(receipts, receipt)
	}
	raw, err := json.Marshal(receipts)
	if err != nil || len(raw) > 1<<20 {
		return nil, errors.Join(err, errors.New("candidate build identity exceeds its bound"))
	}
	return raw, nil
}

func nodeModuleReceipt(module *runtimedebug.Module) nodeBuildModule {
	receipt := nodeBuildModule{Path: module.Path, Version: module.Version, Sum: module.Sum}
	if module.Replace != nil {
		receipt.Replace = module.Replace.Path + "@" + module.Replace.Version + ":" + module.Replace.Sum
	}
	return receipt
}
