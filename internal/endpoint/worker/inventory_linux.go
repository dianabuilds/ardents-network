//go:build linux

package worker

import "strings"

// Inventory selects one of two root-installed artifacts. It is private
// to Endpoint; no local Application request can select an inventory or path.
type Inventory uint8

const (
	Text Inventory = iota
	Stream
)

func (inventory Inventory) prefix() string {
	if inventory == Stream {
		return "ardents-stream-qualification"
	}
	return "ardents-text"
}

func (inventory Inventory) root() string {
	if inventory == Stream {
		return "/usr/lib/ardents/network-stream-worker-root"
	}
	return workerRoot
}

func (inventory Inventory) manifest() string {
	if inventory == Stream {
		return "/etc/ardents/network-stream-worker-artifact.json"
	}
	return artifactPath
}

func (inventory Inventory) schema() string {
	if inventory == Stream {
		return "ardents-network-stream-worker-artifact-v1"
	}
	return "ardents-text-worker-artifact-v1"
}

func (inventory Inventory) rule() string {
	if inventory == Stream {
		return "/usr/share/polkit-1/rules.d/50-ardents-stream-qualification.rules"
	}
	return stopRulePath
}

func (inventory Inventory) socket(role string) string {
	if inventory == Stream {
		return "/run/ardents-stream-qualification/" + role + ".sock"
	}
	return "/run/ardents-text/" + role + ".sock"
}

func (inventory Inventory) user(role string) string {
	prefix := "ardtxt-"
	if inventory == Stream {
		prefix = "ardstq-"
	}
	if role == "publisher" {
		return prefix + "p-"
	}
	return prefix + "r-"
}

func OfUnit(name string) Inventory {
	if strings.HasPrefix(name, Stream.prefix()+"-") {
		return Stream
	}
	return Text
}
