package transport

import "strings"

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return append([]string(nil), in...)
}

func mergeUniqueStrings(groups ...[]string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, group := range groups {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, exists := seen[item]; exists {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func newEndpointObservations(endpoints []string, usable bool) map[string]endpointObservation {
	observed := make(map[string]endpointObservation, len(endpoints))
	for _, endpoint := range endpoints {
		observed[endpoint] = endpointObservation{usable: usable}
	}
	return observed
}

type stringAddress interface {
	String() string
}

func stringifyListenAddresses[T stringAddress](addrs []T) []string {
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.String())
	}
	return out
}
