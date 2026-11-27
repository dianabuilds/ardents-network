package selection

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

// storeGrammar parameterizes the one durable generation-store mechanism shared
// by the closed Entry set and the local Interior set. Each owner keeps its own
// marker, record schema, validator, root lease and refusal prose; the mechanism
// (bounded content-addressed read, strict single-object decode, exhaustion guard
// on the retained generation, staged commit, watermark floor and pruning) is
// identical. It grants no authority and merges no lease.
type storeGrammar[T any] struct {
	markerName, markerContent string
	generation                func(T) uint64
	bind                      func(T, uint64, string) T
	validate                  func(T) bool
	rootBound, rootEntry      string
	marker                    string
	genName, genRead, state   string
	exhausted, commit, bound  string
}

// The grammars are constructed on demand rather than held as package-level
// vars so the owner validators stay reachable only through the store operations
// (Linux-native composition), preserving each platform's reviewed dead profile.
func closedSetGrammar() storeGrammar[closedSetState] {
	return storeGrammar[closedSetState]{
		markerName: closedSetMarker, markerContent: "ardents-entry-set-v1\n",
		generation: func(s closedSetState) uint64 { return s.Generation },
		bind: func(s closedSetState, generation uint64, previous string) closedSetState {
			s.Generation = generation
			s.Previous = previous
			return s
		},
		validate:  validClosedSetState,
		rootBound: "closed Entry root bound exceeded", rootEntry: "closed Entry root entry unavailable",
		marker:  "closed Entry root marker unavailable",
		genName: "closed Entry generation name invalid", genRead: "closed Entry generation invalid", state: "closed Entry state invalid",
		exhausted: "closed Entry generation exhausted", commit: "closed Entry commit invalid", bound: "closed Entry state bound exceeded",
	}
}

func interiorGrammar() storeGrammar[interiorRecord] {
	return storeGrammar[interiorRecord]{
		markerName: interiorMarker, markerContent: "ardents-interior-set-v1\n",
		generation: func(s interiorRecord) uint64 { return s.Generation },
		bind: func(s interiorRecord, generation uint64, previous string) interiorRecord {
			s.Generation = generation
			s.Previous = previous
			return s
		},
		validate:  validInteriorRecord,
		rootBound: "interior root bound exceeded", rootEntry: "interior root entry unavailable",
		marker:  "interior marker unavailable",
		genName: "interior generation invalid", genRead: "interior generation unavailable", state: "interior record invalid",
		exhausted: "interior generation exhausted", commit: "interior commit invalid", bound: "interior record exceeds bound",
	}
}

// inspectStoreRoot validates the bounded owner-only entry inventory and the
// exact ownership marker for one claimed generation root.
func inspectStoreRoot[T any](root string, g storeGrammar[T]) error {
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) > 10 {
		return errors.New(g.rootBound)
	}
	for _, entry := range entries {
		name := entry.Name()
		allowed := name == g.markerName || name == rootLockName || name == "current" || name == "watermark" ||
			strings.HasPrefix(name, "state-") || strings.HasPrefix(name, ".stage-") || strings.HasPrefix(name, ".current-") || strings.HasPrefix(name, ".watermark-")
		info, err := os.Lstat(filepath.Join(root, name))
		if err != nil || !allowed || !info.Mode().IsRegular() {
			return errors.New(g.rootEntry)
		}
	}
	marker, err := readBounded(filepath.Join(root, g.markerName), 64)
	if err != nil || string(marker) != g.markerContent {
		return errors.New(g.marker)
	}
	return nil
}

// readStoreGeneration reads one content-addressed generation, verifying its
// digest, strict single-object schema and owner validator. A wrapped io.EOF
// after the single object is accepted (errors.Is), so trailing data is refused
// uniformly for both owners (G6).
func readStoreGeneration[T any](root, name string, g storeGrammar[T]) (T, error) {
	var zero T
	if !stateName.MatchString(name) {
		return zero, errors.New(g.genName)
	}
	raw, err := readBounded(filepath.Join(root, "state-"+name), maximumStateBytes)
	if err != nil || sha256Hex(raw) != name {
		return zero, errors.New(g.genRead)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var record T
	if err := decoder.Decode(&record); err != nil {
		return zero, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || !g.validate(record) {
		return zero, errors.New(g.state)
	}
	return record, nil
}

// commitStoreGeneration durably commits the next generation. The exhaustion
// guard observes the RETAINED generation (the value about to be incremented),
// not the caller's next value which is immediately overwritten (G2).
func commitStoreGeneration[T any](root string, retained T, retainedName string, next T, g storeGrammar[T]) (T, string, error) {
	var zero T
	if g.generation(retained) == ^uint64(0) {
		return zero, "", errors.New(g.exhausted)
	}
	next = g.bind(next, g.generation(retained)+1, retainedName)
	if !g.validate(next) {
		return zero, "", errors.New(g.commit)
	}
	raw, err := json.Marshal(next)
	if err != nil || len(raw) > maximumStateBytes {
		return zero, "", errors.New(g.bound)
	}
	name := sha256Hex(raw)
	if err := writeGeneration(root, filepath.Join(root, "state-"+name), raw); err != nil {
		return zero, "", err
	}
	if _, err := readStoreGeneration(root, name, g); err != nil {
		return zero, "", err
	}
	if err := durablefile.ReplaceWatermark(root, g.generation(next), name, "entry generation watermark value is invalid"); err != nil {
		return zero, "", err
	}
	if err := replaceCurrent(root, name); err != nil {
		return zero, "", err
	}
	if err := cleanupGenerations(root, name, retainedName); err != nil {
		return zero, "", err
	}
	return next, name, nil
}

// pointerViolatesFloor reports whether a current pointer targets a generation
// beyond the retained watermark floor, or the floor generation under a different
// name. Both owners enforce the identical rule.
func pointerViolatesFloor(pointedGeneration uint64, pointedName string, floor generationWatermark) bool {
	return pointedGeneration > floor.generation || pointedGeneration == floor.generation && pointedName != floor.name
}
