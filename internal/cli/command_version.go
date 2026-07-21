package cli

import (
	"encoding/json"
	"io"

	"ardents/internal/buildinfo"
	"ardents/internal/cli/output"
)

func renderVersion(w io.Writer, outputMode string) int {
	info := buildinfo.Current()
	if outputMode == "json" {
		encoded, err := json.Marshal(info)
		if err != nil {
			return 1
		}
		output.Writeln(w, string(encoded))
		return 0
	}
	output.Writef(w, "ardentsctl %s (%s) %s/%s built %s with %s\n",
		info.Version, info.Commit, info.OS, info.Arch, info.BuildDate, info.GoVersion)
	return 0
}
