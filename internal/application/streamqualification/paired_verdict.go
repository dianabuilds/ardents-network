//go:build linux

package streamqualification

import "fmt"

// PairedReports contains each independently observed Endpoint result. Local
// accepted writes become delivered evidence only when the peer reports the
// same bytes on the matching authenticated Service stream.
type PairedReports struct {
	Readers   [4]Report
	Publisher Report
}

func EvaluatePairedConditionWorkload(reports PairedReports, profile Profile, condition NetworkCondition) []Criterion {
	criteria := EvaluateConditionWorkload(reports.Publisher, PublisherRole, profile, condition)
	for index := range criteria {
		criteria[index].Name = "publisher/" + criteria[index].Name
	}
	published := make(map[uint32]StreamMeasurement, len(reports.Publisher.Streams))
	for _, stream := range reports.Publisher.Streams {
		published[stream.ID] = stream
	}
	seen := make(map[uint32]bool, 256)
	for reader, report := range reports.Readers {
		local := EvaluateConditionWorkload(report, ReaderRole, profile, condition)
		for index := range local {
			local[index].Name = fmt.Sprintf("reader-%d/%s", reader, local[index].Name)
		}
		criteria = append(criteria, local...)
		for _, stream := range report.Streams {
			peer, present := published[stream.ID]
			member := stream.ID <= 127 && stream.ID >= uint32(1+reader*32) && stream.ID <= uint32(31+reader*32) ||
				stream.ID >= uint32(129+reader*96) && stream.ID <= uint32(223+reader*96)
			matched := member && present && !seen[stream.ID] && stream.Tx == peer.Rx && stream.Rx == peer.Tx
			criteria = append(criteria, Criterion{
				Name:     fmt.Sprintf("reader-%d/stream-%d-peer-delivery", reader, stream.ID),
				Observed: float64(stream.Tx + stream.Rx), Relation: "exact peer counters", Bound: float64(peer.Tx + peer.Rx), Passed: matched,
			})
			seen[stream.ID] = true
		}
	}
	criteria = append(criteria, Criterion{Name: "paired-retained-streams", Observed: float64(len(seen)), Relation: "=", Bound: 256, Passed: len(seen) == 256})
	return criteria
}
