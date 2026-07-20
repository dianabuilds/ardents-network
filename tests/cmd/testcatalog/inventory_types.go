package main

type inventoryReport struct {
	Summary   inventorySummary    `json:"summary"`
	Tests     []inventoryTest     `json:"tests"`
	Scenarios []inventoryScenario `json:"scenarios"`
}

type inventorySummary struct {
	TestCount                int `json:"test_count"`
	ScenarioCount            int `json:"scenario_count"`
	FormalBindingCount       int `json:"formal_binding_count"`
	MissingBindingCount      int `json:"missing_binding_count"`
	MissingDocCount          int `json:"missing_doc_count"`
	ScenarioWithoutTestCount int `json:"scenario_without_test_count"`
	IssueCount               int `json:"issue_count"`
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

type parsedTest struct {
	Package       string
	TestName      string
	File          string
	Layer         string
	Domain        string
	ScenarioID    string
	BindingSource string
}
