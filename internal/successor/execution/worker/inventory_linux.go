//go:build linux

package worker

// Ordinary Execution has one closed root-installed Text inventory. A caller
// chooses only its admitted local surface; Qualification owns other artifacts.
const (
	workerPrefix = "ardents-text"
	workerRoot   = "/usr/lib/ardents/text-worker-root"
	artifactPath = "/etc/ardents/text-worker-artifact.json"
	stopRulePath = "/usr/share/polkit-1/rules.d/50-ardents-text.rules"
)

func workerSocket(role string) string { return "/run/ardents-text/" + role + ".sock" }

func workerUserPrefix(role string) string {
	if role == "publisher" {
		return "ardtxt-p-"
	}
	return "ardtxt-r-"
}
