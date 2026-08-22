package release

import (
	"fmt"
	"strings"
)

const h3CustodyNotice = "H3 threshold identities and both rebuild records are project-controlled; no independent custody or builder claim is made"

func detailInvalidMessage(err error) string {
	if err == nil {
		return "release metadata is invalid"
	}
	text := err.Error()
	switch {
	case strings.Contains(text, "expired"):
		return "metadata is expired"
	case strings.Contains(text, "version"):
		return "metadata version is not authorized"
	case strings.Contains(text, "threshold"):
		return "metadata signature threshold is not met"
	case strings.Contains(text, "length"):
		return "metadata length is not authorized"
	case strings.Contains(text, "hash"):
		return "metadata hash is not authorized"
	case strings.Contains(text, "delegat"):
		return "delegated targets are disabled"
	case strings.Contains(text, "root"):
		return "trusted root is not authorized"
	default:
		return "release metadata is invalid"
	}
}

func reject(outcome Outcome, notice string, cause error) Decision {
	if cause != nil {
		notice = fmt.Sprintf("%s: %v", notice, cause)
	}
	return Decision{Outcome: outcome, Notice: notice, CustodyNotice: h3CustodyNotice}
}
