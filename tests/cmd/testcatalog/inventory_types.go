package main

type inventoryReport struct {
	Summary      inventorySummary       `json:"summary"`
	Tests        []inventoryTest        `json:"tests"`
	Scenarios    []inventoryScenario    `json:"scenarios"`
	Requirements []inventoryRequirement `json:"requirements"`
}

type inventorySummary struct {
	TestCount                int `json:"test_count"`
	ScenarioCount            int `json:"scenario_count"`
	FormalBindingCount       int `json:"formal_binding_count"`
	MissingBindingCount      int `json:"missing_binding_count"`
	MissingDocCount          int `json:"missing_doc_count"`
	ScenarioWithoutTestCount int `json:"scenario_without_test_count"`
	IssueCount               int `json:"issue_count"`
	RequirementCount         int `json:"requirement_count"`
	CoveredRequirementCount  int `json:"covered_requirement_count"`
	BlockedRequirementCount  int `json:"blocked_requirement_count"`
	RequirementIssueCount    int `json:"requirement_issue_count"`
}

type inventoryTest struct {
	Package            string   `json:"package"`
	TestName           string   `json:"test_name"`
	File               string   `json:"file"`
	Layer              string   `json:"layer"`
	Domain             string   `json:"domain,omitempty"`
	ScenarioID         string   `json:"scenario_id,omitempty"`
	BindingSource      string   `json:"binding_source"`
	DocumentedScenario string   `json:"documented_scenario_id,omitempty"`
	DocPath            string   `json:"doc_path,omitempty"`
	Issues             []string `json:"issues,omitempty"`
}

type inventoryScenario struct {
	ScenarioID   string   `json:"scenario_id"`
	Layer        string   `json:"layer"`
	Domain       string   `json:"domain,omitempty"`
	DocPath      string   `json:"doc_path"`
	RelatedTests []string `json:"related_tests,omitempty"`
	MatchedTests []string `json:"matched_tests,omitempty"`
	Issues       []string `json:"issues,omitempty"`
}

type scenarioDoc struct {
	ScenarioID        string
	Layer             string
	Domain            string
	DocPath           string
	RelatedTests      []string
	FalsePositiveRisk bool
	FalseNegativeRisk bool
}

type inventoryRequirement struct {
	ID             string   `json:"id"`
	Source         string   `json:"source"`
	Section        string   `json:"section"`
	Status         string   `json:"status"`
	Scenarios      []string `json:"scenarios,omitempty"`
	StaticEvidence []string `json:"static_evidence,omitempty"`
	BlockedReason  string   `json:"blocked_reason,omitempty"`
	Issues         []string `json:"issues,omitempty"`
}

type requirementCoverageFile struct {
	Version      int                    `json:"version"`
	Requirements []inventoryRequirement `json:"requirements"`
}

type parsedTest struct {
	Package       string
	TestName      string
	File          string
	Layer         string
	Domain        string
	ScenarioID    string
	BindingSource string
}
