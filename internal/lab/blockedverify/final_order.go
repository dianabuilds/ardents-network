package blockedverify

import "fmt"

func requiredFinalCellOrder() []string {
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
	for _, identity := range expectedFinalEventSequence() {
		result = append(result, "hostile/"+identity.id)
	}
	return result
}

func requiredFinalMutationCampaigns() []finalMutationCampaign {
	variants := []struct{ id, group, variant string }{
		{"collector-loss", "G8-lifecycle", "collector-loss"},
		{"blocker-loss", "G8-lifecycle", "blocker-loss"},
		{"pipeline-contamination-invite", "G9-ledger-leakage", "pipeline-contamination-invite"},
		{"pipeline-contamination-address", "G9-ledger-leakage", "pipeline-contamination-address"},
		{"pipeline-contamination-path", "G9-ledger-leakage", "pipeline-contamination-path"},
		{"pipeline-contamination-certificate", "G9-ledger-leakage", "pipeline-contamination-certificate"},
	}
	result := make([]finalMutationCampaign, 0, len(variants))
	for _, item := range variants {
		campaign := finalMutationCampaign{ID: item.id, ExpectedVerdict: "invalid"}
		for episode := range 5 {
			campaign.CellOrder = append(campaign.CellOrder,
				fmt.Sprintf("hostile/%s/%s/%d", item.group, item.variant, episode))
		}
		result = append(result, campaign)
	}
	return result
}

func finalMutationCampaignIdentities(values []finalMutationCampaign) []finalMutationCampaign {
	result := make([]finalMutationCampaign, len(values))
	for index, value := range values {
		result[index] = finalMutationCampaign{ID: value.ID, CellOrder: value.CellOrder,
			ExpectedVerdict: value.ExpectedVerdict}
	}
	return result
}
