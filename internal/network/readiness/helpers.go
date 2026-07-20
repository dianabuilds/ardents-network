package readiness

func appendUniqueCapabilities(base []string, items ...string) []string {
	seen := make(map[string]struct{}, len(base)+len(items))
	out := make([]string, 0, len(base)+len(items))
	for _, item := range append(cloneStrings(base), items...) {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return append([]string(nil), in...)
}
