package counters

import "errors"

// These outcomes retain the stable categories used by Hosting.
var (
	ErrUnsupportedPlatform = errors.New("unsupported-platform")
	ErrUnavailable         = errors.New("budget-unavailable")
)

// Interface contains counters from one observed kernel interface identity.
type Interface struct {
	Name  string `json:"name"`
	Index uint64 `json:"index"`
	Tx    uint64 `json:"tx"`
	Rx    uint64 `json:"rx"`
}

// Reading retains the boot identity and ordered interface counters.
type Reading struct {
	Boot       string      `json:"boot"`
	Interfaces []Interface `json:"interfaces"`
}
