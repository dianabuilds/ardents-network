package routing

type Snapshot struct {
	Outcome    string
	Reason     string
	Selected   *Candidate
	Candidates int
	Usable     int
}

type State struct {
	state  string
	reason string
	last   Snapshot
}

func NewState() *State {
	return &State{state: "new"}
}

func (r *State) Ready() {
	r.state = "ready"
	r.reason = ""
}

func (r *State) Degrade(reason string) {
	r.state = "degraded"
	r.reason = reason
}

func (r *State) State() string {
	return r.state
}

func (r *State) Reason() string {
	return r.reason
}

func (r *State) Last() Snapshot {
	return r.last
}

func (r *State) Preview(candidates []Candidate) Snapshot {
	return selectRoute(candidates)
}

func (r *State) PreviewUnavailable(reason string) Snapshot {
	return Snapshot{
		Outcome: "not_found",
		Reason:  reason,
	}
}

func (r *State) Select(candidates []Candidate) Snapshot {
	route := selectRoute(candidates)
	if route.Selected == nil || !route.Selected.Usable {
		r.last = route
		r.Degrade(route.Reason)
		return route
	}
	r.last = route
	r.Ready()
	return route
}

func (r *State) SelectUnavailable(reason string) Snapshot {
	route := Snapshot{
		Outcome: "not_found",
		Reason:  reason,
	}
	r.last = route
	r.Degrade(reason)
	return route
}

func scoreCandidate(candidate Candidate) int {
	score := 0
	if candidate.Trusted {
		score += 100
	}
	if candidate.Usable {
		score += 50
	}
	score += candidate.Reliability * 10
	score += candidate.Privacy * 4
	score -= candidate.Cost * 3
	return score
}

func selectRoute(candidates []Candidate) Snapshot {
	route := Snapshot{Outcome: "not_found", Candidates: len(candidates)}
	bestIndex := -1
	bestScore := -1

	for i, candidate := range candidates {
		if candidate.Usable {
			route.Usable++
		}
		score := scoreCandidate(candidate)
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}

	if bestIndex < 0 {
		route.Reason = "no candidates"
		return route
	}

	selected := candidates[bestIndex]
	route.Selected = &selected
	if !selected.Usable {
		route.Outcome = "not_usable"
		route.Reason = "best candidate is not usable"
		return route
	}

	route.Outcome = "usable"
	route.Reason = "candidate selected"
	return route
}
