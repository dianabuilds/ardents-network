// Package buildinfo owns immutable build and version identity.
// It does not own runtime health or release orchestration.
package buildinfo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"runtime"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

func Current() Info {
	return Info{
		Version: Version, Commit: Commit, BuildDate: BuildDate,
		GoVersion: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH,
	}
}

// Fingerprint is the stable opaque identity shared by the executable version
// output and instance-bound operational probes.
func Fingerprint() string {
	encoded, err := json.Marshal(Current())
	if err != nil {
		panic("encode build identity: " + err.Error())
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
