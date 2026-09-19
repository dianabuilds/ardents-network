//go:build linux

package endpoint

import "strings"

// workerInventory selects one of two root-installed artifacts. It is private
// to Endpoint; no local Application request can select an inventory or path.
type workerInventory uint8

const (
	textInventory workerInventory = iota
	streamInventory
)

func (inventory workerInventory) prefix() string {
	if inventory == streamInventory {
		return "ardents-stream-qualification"
	}
	return "ardents-text"
}

func (inventory workerInventory) root() string {
	if inventory == streamInventory {
		return "/usr/lib/ardents/network-stream-worker-root"
	}
	return textWorkerRoot
}

func (inventory workerInventory) manifest() string {
	if inventory == streamInventory {
		return "/etc/ardents/network-stream-worker-artifact.json"
	}
	return textWorkerArtifactPath
}

func (inventory workerInventory) schema() string {
	if inventory == streamInventory {
		return "ardents-network-stream-worker-artifact-v1"
	}
	return "ardents-text-worker-artifact-v1"
}

func (inventory workerInventory) rule() string {
	if inventory == streamInventory {
		return "/usr/share/polkit-1/rules.d/50-ardents-stream-qualification.rules"
	}
	return textWorkerStopRulePath
}

func (inventory workerInventory) socket(role string) string {
	if inventory == streamInventory {
		return "/run/ardents-stream-qualification/" + role + ".sock"
	}
	return "/run/ardents-text/" + role + ".sock"
}

func (inventory workerInventory) user(role string) string {
	prefix := "ardtxt-"
	if inventory == streamInventory {
		prefix = "ardstq-"
	}
	if role == "publisher" {
		return prefix + "p-"
	}
	return prefix + "r-"
}

func inventoryOfUnit(name string) workerInventory {
	if strings.HasPrefix(name, streamInventory.prefix()+"-") {
		return streamInventory
	}
	return textInventory
}
