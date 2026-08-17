package blockedentry

import "fmt"

func finalCellOrder() []string {
	var result []string
	floors := []struct {
		id    string
		count int
	}{{"C0", 20}, {"C1", 20}, {"C2", 20}, {"C3", 5}, {"C4", 5}, {"C5", 20}, {"C6", 20}}
	for _, profile := range floors {
		for episode := range profile.count {
			result = append(result, fmt.Sprintf("profile/%s/%02d", profile.id, episode))
		}
	}
	for _, profile := range []string{"h3-s5-b1-v1", "h3-s5-b1-v1-strong"} {
		for batch := range 5 {
			result = append(result, fmt.Sprintf("capacity/%s/%d", profile, batch))
		}
	}
	for _, direction := range []string{"endpoint-to-publisher", "publisher-to-endpoint"} {
		result = append(result, "sustained/"+direction+"/direct-before")
		for run := range 5 {
			result = append(result, fmt.Sprintf("sustained/%s/run-%d", direction, run))
		}
		result = append(result, "sustained/"+direction+"/direct-after")
	}
	for cell := range 5 {
		result = append(result, fmt.Sprintf("pressure/P%d", cell))
	}
	for episode := range 5 {
		result = append(result, fmt.Sprintf("recovery/%d", episode))
	}
	for _, group := range hostileMatrix() {
		for _, variant := range group.Variants {
			for episode := range 5 {
				result = append(result, "hostile/"+eventID(group.ID, variant, episode))
			}
		}
	}
	return result
}
