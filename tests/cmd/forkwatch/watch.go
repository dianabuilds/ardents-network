package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

type Status string

const (
	StatusUpToDate        Status = "up_to_date"
	StatusUpdateAvailable Status = "update_available"
)

type Manifest struct {
	Name           string `json:"name"`
	ManifestPath   string `json:"manifest_path"`
	Module         string `json:"module"`
	UpstreamSource string `json:"upstream_source"`
	PinnedBaseline string `json:"pinned_baseline"`
}

type Result struct {
	Name            string   `json:"name"`
	ManifestPath    string   `json:"manifest_path"`
	Module          string   `json:"module"`
	UpstreamSource  string   `json:"upstream_source"`
	PinnedBaseline  string   `json:"pinned_baseline"`
	LatestStableTag string   `json:"latest_stable_tag"`
	NewerStableTags []string `json:"newer_stable_tags"`
	Status          Status   `json:"status"`
}

func loadForkManifests(forksDir string) ([]Manifest, error) {
	entries, err := os.ReadDir(forksDir)
	if err != nil {
		return nil, fmt.Errorf("read forks dir: %w", err)
	}

	manifests := make([]Manifest, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(forksDir, entry.Name(), "FORK.md")
		manifest, err := parseManifest(manifestPath)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, manifest)
	}

	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Name < manifests[j].Name
	})
	return manifests, nil
}

func parseManifest(path string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("open manifest %s: %w", path, err)
	}
	defer file.Close()

	manifest := Manifest{
		Name:         filepath.Base(filepath.Dir(path)),
		ManifestPath: filepath.ToSlash(path),
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "- `Module`:"):
			manifest.Module = extractManifestValue(line)
		case strings.HasPrefix(line, "- `Upstream source`:"):
			manifest.UpstreamSource = extractManifestValue(line)
		case strings.HasPrefix(line, "- `Pinned upstream baseline`:"):
			manifest.PinnedBaseline = extractManifestValue(line)
		}
	}
	if err := scanner.Err(); err != nil {
		return Manifest{}, fmt.Errorf("scan manifest %s: %w", path, err)
	}

	if manifest.Module == "" || manifest.UpstreamSource == "" || manifest.PinnedBaseline == "" {
		return Manifest{}, fmt.Errorf("manifest %s is missing required fields", path)
	}
	if !semver.IsValid(manifest.PinnedBaseline) {
		return Manifest{}, fmt.Errorf("manifest %s has non-semver baseline %q", path, manifest.PinnedBaseline)
	}
	return manifest, nil
}

func extractManifestValue(line string) string {
	idx := strings.Index(line, ":")
	if idx == -1 {
		return ""
	}
	value := strings.TrimSpace(line[idx+1:])
	return strings.Trim(value, "`")
}

func checkManifest(manifest Manifest) (Result, error) {
	tags, err := upstreamStableTags(manifest.UpstreamSource)
	if err != nil {
		return Result{}, fmt.Errorf("query upstream tags for %s: %w", manifest.Name, err)
	}
	if len(tags) == 0 {
		return Result{}, fmt.Errorf("no stable semver tags found for %s", manifest.Name)
	}

	newer := make([]string, 0)
	for _, tag := range tags {
		if semver.Compare(tag, manifest.PinnedBaseline) > 0 {
			newer = append(newer, tag)
		}
	}

	status := StatusUpToDate
	if len(newer) > 0 {
		status = StatusUpdateAvailable
	}

	return Result{
		Name:            manifest.Name,
		ManifestPath:    manifest.ManifestPath,
		Module:          manifest.Module,
		UpstreamSource:  manifest.UpstreamSource,
		PinnedBaseline:  manifest.PinnedBaseline,
		LatestStableTag: tags[len(tags)-1],
		NewerStableTags: newer,
		Status:          status,
	}, nil
}

func upstreamStableTags(source string) ([]string, error) {
	cmd := exec.Command("git", "ls-remote", "--tags", "--refs", source)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	tags := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		ref := fields[1]
		if !strings.HasPrefix(ref, "refs/tags/") {
			continue
		}
		tag := strings.TrimPrefix(ref, "refs/tags/")
		if !semver.IsValid(tag) || semver.Prerelease(tag) != "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sort.Slice(tags, func(i, j int) bool {
		return semver.Compare(tags[i], tags[j]) < 0
	})
	return tags, nil
}
