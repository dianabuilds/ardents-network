package namelease

import (
	"errors"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

func validateParents(child string, parents []Record, now int64) (*Record, error) {
	if len(parents) == 0 {
		if strings.Contains(child, ".") {
			return nil, errors.New("hierarchical Service Name requires its parent lineage")
		}
		return nil, nil
	}
	childName, err := naming.Parse(child)
	if err != nil {
		return nil, err
	}
	for i := range parents {
		if err := validateRecord(parents[i]); err != nil {
			return nil, errors.New("parent lineage contains an invalid record")
		}
		parentName, parseErr := naming.Parse(parents[i].Name)
		if parseErr != nil || !naming.IsDescendant(childName, parentName) {
			return nil, errors.New("parent lineage does not contain the child")
		}
		if i > 0 && (parents[i-1].ParentName != parents[i].Name ||
			parents[i-1].ParentGeneration != parents[i].Generation) {
			return nil, errors.New("parent lineage generation is discontinuous")
		}
		if ok, _ := liveLease(parents[i], now); !ok {
			return nil, errors.New("parent lineage is not resolvable")
		}
		childName = parentName
	}
	if parents[len(parents)-1].ParentName != "" {
		return nil, errors.New("parent lineage does not reach a root")
	}
	return &parents[0], nil
}

func sameParent(record *Record, parent *Record) bool {
	if record.ParentName == "" {
		return parent == nil && record.ParentGeneration == 0
	}
	return parent != nil && record.ParentName == parent.Name &&
		record.ParentGeneration == parent.Generation
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
