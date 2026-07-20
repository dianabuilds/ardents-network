package route

type Candidate struct {
	Subject     string
	Service     string
	Endpoint    string
	Scheme      string
	Mode        string
	Trusted     bool
	Usable      bool
	Cost        int
	Privacy     int
	Reliability int
}

func BuildCandidates(subject, service, mode string, endpoints []string, trusted bool, usable func(string) bool) []Candidate {
	items := make([]Candidate, 0, len(endpoints))
	for _, endpoint := range endpoints {
		scheme := endpointScheme(endpoint)
		items = append(items, Candidate{
			Subject:     subject,
			Service:     service,
			Endpoint:    endpoint,
			Scheme:      scheme,
			Mode:        mode,
			Trusted:     trusted,
			Usable:      usable != nil && usable(endpoint),
			Cost:        candidateCost(scheme),
			Privacy:     candidatePrivacy(scheme),
			Reliability: candidateReliability(scheme),
		})
	}
	return items
}

func endpointScheme(endpoint string) string {
	if len(endpoint) > 0 && endpoint[0] == '/' {
		return "multiaddr"
	}
	for i := 0; i+2 < len(endpoint); i++ {
		if endpoint[i:i+3] == "://" {
			return endpoint[:i]
		}
	}
	return "unknown"
}

func candidateCost(scheme string) int {
	switch scheme {
	case "quic", "multiaddr":
		return 1
	case "tcp":
		return 2
	case "relay":
		return 3
	default:
		return 4
	}
}

func candidatePrivacy(scheme string) int {
	switch scheme {
	case "relay":
		return 4
	case "quic", "multiaddr":
		return 3
	default:
		return 2
	}
}

func candidateReliability(scheme string) int {
	switch scheme {
	case "quic", "multiaddr":
		return 5
	case "tcp":
		return 4
	case "relay":
		return 3
	default:
		return 1
	}
}
