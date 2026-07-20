package cli

import "strings"

func joinCSV(items []string) string {
	return strings.Join(items, ", ")
}
