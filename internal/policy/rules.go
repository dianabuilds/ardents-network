package policy

import (
	"slices"
	"strings"
)

func NormalizeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = normalize(item)
		if item == "" || slices.Contains(out, item) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func ContainsNormalized(items []string, want string) bool {
	want = normalize(want)
	for _, item := range items {
		if normalize(item) == want {
			return true
		}
	}
	return false
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
