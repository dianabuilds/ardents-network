package main

import (
	"path/filepath"
	"sort"
)

func buildInventory(patterns []string) (inventoryReport, error) {
	modulePath, err := findModulePath()
	if err != nil {
		return inventoryReport{}, err
	}

	tests, err := collectInventoryTests(patterns, modulePath)
	if err != nil {
		return inventoryReport{}, err
	}

	scenarios, err := collectScenarioDocs()
	if err != nil {
		return inventoryReport{}, err
	}

	docByScenario, docByRelatedTest := inventoryDocIndexes(scenarios)
	report := inventoryReport{
		Tests:     make([]inventoryTest, 0, len(tests)),
		Scenarios: make([]inventoryScenario, 0, len(scenarios)),
	}

	matchedTests := map[string][]string{}
	for _, test := range tests {
		entry := inventoryTestEntry(test, modulePath, docByScenario, docByRelatedTest, matchedTests)
		report.Summary = accumulateInventorySummary(report.Summary, entry)
		report.Tests = append(report.Tests, entry)
	}

	for _, doc := range scenarios {
		entry := inventoryScenarioEntry(doc, matchedTests)
		report.Summary = accumulateScenarioSummary(report.Summary, entry)
		report.Scenarios = append(report.Scenarios, entry)
	}

	sortInventoryReport(&report)
	report.Summary.TestCount = len(report.Tests)
	report.Summary.ScenarioCount = len(report.Scenarios)
	return report, nil
}

func filterInventory(report inventoryReport, layer string, domain string, scenario string) inventoryReport {
	filtered := inventoryReport{}
	for _, test := range report.Tests {
		if layer != "" && test.Layer != layer {
			continue
		}
		if domain != "" && test.Domain != domain {
			continue
		}
		if scenario != "" && test.ScenarioID != scenario && test.DocumentedScenario != scenario {
			continue
		}
		filtered.Tests = append(filtered.Tests, test)
	}
	for _, doc := range report.Scenarios {
		if layer != "" && doc.Layer != layer {
			continue
		}
		if domain != "" && doc.Domain != domain {
			continue
		}
		if scenario != "" && doc.ScenarioID != scenario {
			continue
		}
		filtered.Scenarios = append(filtered.Scenarios, doc)
	}
	filtered.Summary = inventorySummary{
		TestCount:                len(filtered.Tests),
		ScenarioCount:            len(filtered.Scenarios),
		FormalBindingCount:       countTestsBySource(filtered.Tests, "formal"),
		MissingBindingCount:      countTestsBySource(filtered.Tests, "missing"),
		MissingDocCount:          countTestsWithIssue(filtered.Tests, "missing scenario document"),
		ScenarioWithoutTestCount: countScenariosWithIssue(filtered.Scenarios, "scenario doc has no runnable code binding"),
		IssueCount:               countTestIssues(filtered.Tests) + countScenarioIssues(filtered.Scenarios),
	}
	return filtered
}

func inventoryDocIndexes(scenarios []scenarioDoc) (map[string]scenarioDoc, map[string][]scenarioDoc) {
	docByScenario := map[string]scenarioDoc{}
	docByRelatedTest := map[string][]scenarioDoc{}
	for _, doc := range scenarios {
		docByScenario[doc.ScenarioID] = doc
		for _, testRef := range doc.RelatedTests {
			docByRelatedTest[testRef] = append(docByRelatedTest[testRef], doc)
		}
	}
	return docByScenario, docByRelatedTest
}

func inventoryTestEntry(test parsedTest, modulePath string, docByScenario map[string]scenarioDoc, docByRelatedTest map[string][]scenarioDoc, matchedTests map[string][]string) inventoryTest {
	entry := inventoryTest{
		Package:       test.Package,
		TestName:      test.TestName,
		File:          test.File,
		Layer:         test.Layer,
		Domain:        test.Domain,
		ScenarioID:    test.ScenarioID,
		BindingSource: test.BindingSource,
	}

	if test.BindingSource == "missing" {
		entry.Issues = append(entry.Issues, "missing code scenario binding")
	}

	testRef := filepath.ToSlash(filepath.Join(packagePathFromImport(test.Package, modulePath), test.File)) + "::" + test.TestName
	if test.ScenarioID != "" {
		doc, ok := docByScenario[test.ScenarioID]
		if !ok {
			entry.Issues = append(entry.Issues, "missing scenario document")
		} else {
			entry.DocPath = filepath.ToSlash(doc.DocPath)
			if entry.Domain == "" {
				entry.Domain = doc.Domain
			}
			matchedTests[doc.ScenarioID] = append(matchedTests[doc.ScenarioID], testRef)
		}
	}

	fileRef := filepath.ToSlash(filepath.Join(packagePathFromImport(test.Package, modulePath), test.File))
	docs := append([]scenarioDoc{}, docByRelatedTest[testRef]...)
	docs = append(docs, docByRelatedTest[fileRef]...)
	docs = uniqueScenarioDocs(docs)

	switch len(docs) {
	case 1:
		entry = reconcileDocumentedScenario(entry, docs[0])
	case 0:
	default:
		entry.Issues = append(entry.Issues, "test is referenced by multiple scenario docs")
	}

	return entry
}

func reconcileDocumentedScenario(entry inventoryTest, doc scenarioDoc) inventoryTest {
	entry.DocumentedScenario = doc.ScenarioID
	if entry.DocPath == "" {
		entry.DocPath = filepath.ToSlash(doc.DocPath)
	}
	if entry.Domain == "" {
		entry.Domain = doc.Domain
	}
	if entry.ScenarioID == "" {
		entry.Issues = append(entry.Issues, "scenario doc exists but test metadata is missing")
	} else if entry.ScenarioID != doc.ScenarioID {
		entry.Issues = append(entry.Issues, "test scenario id does not match related scenario doc")
	}
	return entry
}

func inventoryScenarioEntry(doc scenarioDoc, matchedTests map[string][]string) inventoryScenario {
	entry := inventoryScenario{
		ScenarioID:   doc.ScenarioID,
		Layer:        doc.Layer,
		Domain:       doc.Domain,
		DocPath:      filepath.ToSlash(doc.DocPath),
		RelatedTests: append([]string{}, doc.RelatedTests...),
		MatchedTests: uniqueStrings(matchedTests[doc.ScenarioID]),
	}
	if len(entry.MatchedTests) == 0 {
		entry.Issues = append(entry.Issues, "scenario doc has no runnable code binding")
	}
	if doc.Layer != "integration" && doc.Layer != "e2e" {
		entry.Issues = append(entry.Issues, "scenario doc layer must be integration or e2e")
	}
	if doc.Domain == "" {
		entry.Issues = append(entry.Issues, "scenario doc is missing domain")
	}
	if !doc.FalsePositiveRisk {
		entry.Issues = append(entry.Issues, "scenario doc is missing non-empty False Positive Risk")
	}
	if !doc.FalseNegativeRisk {
		entry.Issues = append(entry.Issues, "scenario doc is missing non-empty False Negative Risk")
	}
	return entry
}

func accumulateInventorySummary(summary inventorySummary, entry inventoryTest) inventorySummary {
	switch entry.BindingSource {
	case "formal":
		summary.FormalBindingCount++
	default:
		summary.MissingBindingCount++
	}
	if contains(entry.Issues, "missing scenario document") {
		summary.MissingDocCount++
	}
	summary.IssueCount += len(entry.Issues)
	return summary
}

func accumulateScenarioSummary(summary inventorySummary, entry inventoryScenario) inventorySummary {
	if contains(entry.Issues, "scenario doc has no runnable code binding") {
		summary.ScenarioWithoutTestCount++
	}
	summary.IssueCount += len(entry.Issues)
	return summary
}

func sortInventoryReport(report *inventoryReport) {
	sort.Slice(report.Tests, func(i, j int) bool {
		if report.Tests[i].Package == report.Tests[j].Package {
			return report.Tests[i].TestName < report.Tests[j].TestName
		}
		return report.Tests[i].Package < report.Tests[j].Package
	})
	sort.Slice(report.Scenarios, func(i, j int) bool {
		return report.Scenarios[i].ScenarioID < report.Scenarios[j].ScenarioID
	})
}
