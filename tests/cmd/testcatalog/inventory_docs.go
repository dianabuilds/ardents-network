package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func collectScenarioDocs() ([]scenarioDoc, error) {
	roots := []string{"docs/qa/integration", "docs/qa/e2e"}
	var docs []scenarioDoc
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".md") || filepath.Base(path) == "README.md" {
				return nil
			}

			doc, err := parseScenarioDoc(path)
			if err != nil {
				return err
			}
			docs = append(docs, doc)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return docs, nil
}

func parseScenarioDoc(path string) (scenarioDoc, error) {
	file, err := os.Open(path)
	if err != nil {
		return scenarioDoc{}, err
	}
	defer file.Close()

	var current string
	doc := scenarioDoc{DocPath: path}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if key, value, ok := parseFieldLine(line); ok {
			doc = applyDocField(doc, key, value)
			if key == "False Positive Risk" || key == "False Negative Risk" {
				current = key
			}
			continue
		}
		if heading, value := parseHeading(line); heading != "" {
			current = heading
			if heading == "Scenario ID" && value != "" {
				doc.ScenarioID = value
			}
			continue
		}
		doc = applyDocSectionLine(doc, current, line)
	}
	if err := scanner.Err(); err != nil {
		return scenarioDoc{}, err
	}
	if doc.ScenarioID == "" || doc.Layer == "" {
		return scenarioDoc{}, fmt.Errorf("scenario doc %s is missing Scenario ID or Layer", path)
	}
	return doc, nil
}

func applyDocField(doc scenarioDoc, key string, value string) scenarioDoc {
	switch key {
	case "Scenario ID":
		doc.ScenarioID = value
	case "Layer":
		doc.Layer = value
	case "Domain":
		doc.Domain = value
	case "False Positive Risk":
		doc.FalsePositiveRisk = value != ""
	case "False Negative Risk":
		doc.FalseNegativeRisk = value != ""
	}
	return doc
}

func applyDocSectionLine(doc scenarioDoc, current string, line string) scenarioDoc {
	switch current {
	case "Scenario ID":
		if value := parseMarkdownValue(line); value != "" {
			doc.ScenarioID = value
		}
	case "Layer":
		if value := parseMarkdownValue(line); value != "" {
			doc.Layer = value
		}
	case "Domain":
		if value := parseMarkdownValue(line); value != "" {
			doc.Domain = value
		}
	case "Related Tests":
		if ref := parseRelatedTest(line); ref != "" {
			doc.RelatedTests = append(doc.RelatedTests, ref)
		}
	case "False Positive Risk":
		if line != "" {
			doc.FalsePositiveRisk = true
		}
	case "False Negative Risk":
		if line != "" {
			doc.FalseNegativeRisk = true
		}
	}
	return doc
}

func parseHeading(line string) (string, string) {
	if !strings.HasPrefix(line, "#") {
		return "", ""
	}
	value := strings.TrimSpace(strings.TrimLeft(line, "#"))
	if strings.HasPrefix(value, "Scenario ") {
		return "Scenario ID", strings.TrimSpace(strings.TrimPrefix(value, "Scenario "))
	}
	return value, ""
}

func parseMarkdownValue(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if strings.HasPrefix(line, "`") && strings.HasSuffix(line, "`") && len(line) >= 2 {
		return strings.Trim(line, "`")
	}
	return line
}

func parseRelatedTest(line string) string {
	line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
	if line == "" {
		return ""
	}
	if strings.HasPrefix(line, "`") && strings.HasSuffix(line, "`") && len(line) >= 2 {
		return strings.Trim(line, "`")
	}
	return ""
}

func parseFieldLine(line string) (string, string, bool) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
	if line == "" {
		return "", "", false
	}
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := parseMarkdownValue(parts[0])
	value := parseMarkdownValue(parts[1])
	switch key {
	case "Scenario ID", "Layer", "Domain", "Category", "False Positive Risk", "False Negative Risk":
	default:
		return "", "", false
	}
	if key == "" {
		return "", "", false
	}
	if value == "" && key != "False Positive Risk" && key != "False Negative Risk" {
		return "", "", false
	}
	return key, value, true
}
