package stage6verify

import (
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
)

var expectedDecisions = []string{"R-041", "R-042-O1b", "R-043", "R-044-O2", "R-045-O1b", "R-046-O1", "R-047-O1", "R-055-S6E1", "R-057-O1"}

func manifestInventory() []string {
	paths := []string{"campaign.json"}
	for ordinal := range expectedCells {
		paths = append(paths, "cells/"+twoDigits(ordinal)+".json")
	}
	return paths
}

func evidenceInventory() []string {
	paths := []string{"index.json"}
	for ordinal := range expectedCells {
		prefix := "cells/" + twoDigits(ordinal)
		paths = append(paths, prefix+"/cleanup.json", prefix+"/terminal.json",
			prefix+"/observations/trace.jsonl")
	}
	return paths
}

func exactRootInventory(root string, expected []string) error {
	if err := inspectRoot(root); err != nil {
		return err
	}
	actual := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return errors.New("S6E1 root contains a forbidden entry")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		actual = append(actual, filepath.ToSlash(relative))
		return nil
	})
	want := append([]string(nil), expected...)
	sort.Strings(actual)
	sort.Strings(want)
	if err != nil || !equalStrings(actual, want) {
		return errors.New("S6E1 root inventory is not exact")
	}
	return nil
}
