package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
)

type replacementNativeProof struct {
	before, after *initialFile
}

func replacementNativePaths(journal string) (before, after string) {
	return journal + ".native-before", journal + ".native-after"
}

// The before snapshot of a later independently proved reload can retain more
// ancestors, but cannot drop or substitute any previous observation, including
// generators. Only that reload's before/after relation may change generators.
// Complete native birth/control lineage remains mandatory.
func compareContextContinuation(original, current []byte) error {
	before, err := decodeNativeContext(original)
	if err != nil {
		return err
	}
	after, err := decodeNativeContext(current)
	if err != nil {
		return err
	}
	objects := map[string]nativeObject{}
	for _, object := range after.Ancestors {
		objects[object.Path] = object
	}
	for _, object := range before.Ancestors {
		if objects[object.Path] != object {
			return ErrBinding
		}
	}
	for _, path := range before.Absent {
		if !slices.Contains(after.Absent, path) {
			return ErrBinding
		}
	}
	before.Ancestors, before.Absent = nil, nil
	after.Ancestors, after.Absent = nil, nil
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	if !bytes.Equal(a, b) {
		return ErrBinding
	}
	return nil
}

func (c *completedInitial) openRecoveredContext() error {
	raw, err := c.recoveredContextBytes()
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}
	c.files.recoveryContext, err = openRecoveryContext(c.files.ctx, raw)
	if err != nil {
		return err
	}
	return checkRecoveryManager(c.files.recoveryContext)
}

func (c *completedInitial) recoveredContextBytes() ([]byte, error) {
	if c == nil || c.files == nil || len(c.selectionOrder) == 0 {
		return nil, ErrInput
	}
	var raw []byte
	if c.files.recovered != nil {
		raw = c.files.recovered.nativeRaw
	}
	// Prove reload and recovered invocation contexts in actual generation order.
	// An older retirement must precede a later owned generator refresh.
	for n := len(c.selectionOrder) - 1; n >= 0; n-- {
		selected := c.selectionOrder[n]
		if selected.replacement != nil {
			if prefix := c.replacementPreReload(replacementJournal(selected.binding.Request.InstallationRoot, selected.binding.Generation)); prefix != nil {
				if err := prefix.check(); err != nil {
					return nil, err
				}
				if raw != nil {
					if err := compareContextContinuation(raw, prefix.nativeRaw); err != nil {
						return nil, err
					}
				}
				raw = prefix.nativeRaw
			} else if raw == nil {
				for _, recovered := range c.files.recoveredReplacements {
					if recovered.generation == selected.binding.Generation {
						raw = recovered.nativeRaw
					}
				}
			} else {
				proof := c.reloads[selected.binding.Generation]
				if proof == nil || proof.before == nil || proof.after == nil {
					return nil, ErrRepairRequired
				}
				if err := errors.Join(compareContextContinuation(raw, proof.before.body), compareReloadContexts(proof.before.body, proof.after.body)); err != nil {
					return nil, err
				}
				raw = proof.after.body
			}
		}
		if chain := c.files.invocations; chain != nil {
			for _, entry := range chain.entries {
				if entry.selection.binding.Generation != selected.binding.Generation {
					continue
				}
				var next []byte
				if entry.terminal != nil {
					next = entry.terminal.nativeRaw
				} else if entry.retirement != nil && entry.retirement.recovery != nil {
					next = entry.retirement.recovery.nativeRaw
				}
				if next == nil {
					continue
				}
				if raw != nil {
					if err := compareContextContinuation(raw, next); err != nil {
						return nil, err
					}
				}
				raw = next
			}
		}
	}
	return raw, nil
}

// Only actual replacement or its independently admitted pre-Reload recovery
// invokes this after its successful one-use Reload
// and checked fresh namespace. No caller-supplied snapshot enters this path.
func (f *completedFiles) observeOwnedReload(namespace *startNamespace) (returned error) {
	if f == nil {
		return ErrInput
	}
	if f.recoveryContext == nil {
		return nil
	}
	old := f.recoveryContext
	fresh, _, err := old.afterOwnedReload(namespace)
	if err != nil {
		return err
	}
	f.recoveryContext = fresh
	return checkRecoveryManager(f.recoveryContext)
}

// Concrete native observations only. The caller must own and admit the actual
// reload; this method cannot perform or attest a manager effect. The fresh
// observation is withheld until every old physical descriptor has closed.
func (old *recoveryContext) afterOwnedReload(namespace *startNamespace) (*recoveryContext, []byte, error) {
	if old == nil || old.closed || old.terminal != nil {
		return nil, nil, ErrBinding
	}
	if err := errors.Join(old.ctx.Err(), old.identity.check()); err != nil {
		return nil, nil, err
	}
	for _, object := range old.expected.Ancestors {
		if !generatorDirectory(object.Path) {
			if err := observeRecoveryDirectory(old.dirs[object.Path], object); err != nil {
				return nil, nil, err
			}
		}
	}
	original, err := json.Marshal(old.expected)
	if err != nil {
		return nil, nil, err
	}
	raw, err := nativeContextAfterReload(append(original, '\n'), namespace)
	if err != nil {
		return nil, nil, err
	}
	fresh, err := openRecoveryContext(old.ctx, raw)
	if err != nil {
		return nil, nil, err
	}
	if err := old.close(); err != nil {
		return nil, nil, errors.Join(err, fresh.close())
	}
	return fresh, raw, nil
}
