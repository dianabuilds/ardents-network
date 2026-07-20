package data

import "sort"

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func sortedKeys[V any](items map[string]V) []string {
	out := make([]string, 0, len(items))
	for id := range items {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
