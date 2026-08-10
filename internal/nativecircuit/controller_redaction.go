package nativecircuit

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var nativeIPv4Address = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)

func sanitizeNativeFailure(layout nativeRunLayout, message string) string {
	replacements := []struct{ value, label string }{
		{layout.evidenceDir, "<evidence-dir>"},
		{layout.runDirectory, "<run-dir>"},
		{layout.repositoryRoot, "<repository-root>"},
		{filepath.Dir(layout.runDirectory), "<session-root>"},
	}
	slices.SortFunc(replacements, func(left, right struct{ value, label string }) int {
		return len(right.value) - len(left.value)
	})
	for _, replacement := range replacements {
		forward := strings.ReplaceAll(replacement.value, `\`, "/")
		backward := strings.ReplaceAll(replacement.value, "/", `\`)
		for _, value := range []string{replacement.value, forward, backward} {
			if value != "" {
				message = strings.ReplaceAll(message, value, replacement.label)
			}
		}
	}
	return nativeIPv4Address.ReplaceAllString(message, "<address>")
}
