package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"ardents/internal/buildinfo"
)

func renderVersion(w io.Writer, output string) int {
	info := buildinfo.Current()
	if output == "json" {
		encoded, err := json.Marshal(info)
		if err != nil {
			return 1
		}
		_, _ = fmt.Fprintln(w, string(encoded))
		return 0
	}
	_, _ = fmt.Fprintf(w, "ard %s (%s) %s/%s built %s with %s\n",
		info.Version, info.Commit, info.OS, info.Arch, info.BuildDate, info.GoVersion)
	return 0
}
