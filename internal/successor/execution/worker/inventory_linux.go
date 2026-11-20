//go:build linux

package worker

var textInventory = inventory{
	prefix:       "ardents-text",
	root:         "/usr/lib/ardents/text-worker-root",
	manifest:     "/etc/ardents/text-worker-artifact.json",
	schema:       "ardents-text-worker-artifact-v1",
	rule:         "/usr/share/polkit-1/rules.d/50-ardents-text.rules",
	socketDir:    "/run/ardents-text",
	userPrefix:   "ardtxt-",
	slice:        "system.slice",
	cgroupPrefix: "/system.slice/",
}

var streamInventory = inventory{
	prefix:     "ardents-stream-qualification",
	root:       "/usr/lib/ardents/network-stream-worker-root",
	manifest:   "/etc/ardents/network-stream-worker-artifact.json",
	schema:     "ardents-network-stream-worker-artifact-v1",
	rule:       "/usr/share/polkit-1/rules.d/50-ardents-stream-qualification.rules",
	socketDir:  "/run/ardents-stream-qualification",
	userPrefix: "ardstq-",
	slice:      "ardents-qualification-owner.slice",
	// The qualification owner slice nests under the Ardents qualification
	// hierarchy (systemd derives the cgroup path from the slice name); the
	// worker's Slice= leaf name alone does not describe its kernel cgroup.
	cgroupPrefix: "/ardents.slice/ardents-qualification.slice/ardents-qualification-owner.slice/",
}

func (inv inventory) socket(role string) string { return inv.socketDir + "/" + role + ".sock" }

func (inv inventory) user(role string) string {
	if role == "publisher" {
		return inv.userPrefix + "p-"
	}
	return inv.userPrefix + "r-"
}
