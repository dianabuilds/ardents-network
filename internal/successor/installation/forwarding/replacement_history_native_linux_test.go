//go:build linux && network_signing_native

package forwarding

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// This failure-only probe stops after the actual retirement leaf read. Detached
// byte vectors never supply native history, retirement, Release or Start rights.
func TestForwardingRetirementSnapshotNativeReplacementHistoryParent(t *testing.T) {
	for _, fault := range []string{"bound-parent", "wrong-parent-group", "root-parent-group", "writable-parent", "wrong-parent-owner", "wrong-leaf-group", "symlink-leaf"} {
		t.Run(fault, func(t *testing.T) { replacementHistoryRetirementParent(t, fault) })
	}
}

func replacementHistoryRetirementParent(t *testing.T, fault string) {
	t.Helper()
	if os.Geteuid() != 0 || os.Getegid() != 0 {
		t.Fatal("root native profile required")
	}
	base, err := os.MkdirTemp(os.TempDir(), "rh-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(base); e != nil {
			t.Error(e)
		}
	})
	var fs syscall.Statfs_t
	if err := syscall.Statfs(base, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("native ext4 required", err)
	}
	rows, selected, previous, _, before, after := replacementContextVector(t)
	_, _, bodies := selectedGenerationVector(t)
	oldRoot, oldHistory := selected.binding.Request.InstallationRoot, selected.binding.History
	root, history := filepath.Join(base, "n"), filepath.Join(base, "h")
	encode := func(v any) []byte {
		raw, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		return append(raw, '\n')
	}
	for _, s := range []*generationSelection{&previous, &selected} {
		s.binding.Request.InstallationRoot = root
		s.binding.History = history
		request := encode(s.binding.Request)
		s.binding.RequestSHA256 = digest(request)
		unit, e := RenderUnit([]byte(Template), s.binding.Request, s.binding.Generation)
		if e != nil {
			t.Fatal(e)
		}
		s.binding.UnitSHA256 = digest(unit)
	}
	previous.raw = encode(previous.binding)
	selected.replacement.Configuration = selected.binding
	selected.replacement.PredecessorBindingSHA256 = digest(previous.raw)
	selected.raw = encode(selected.replacement)
	if _, e := decodeSelection(previous.raw); e != nil {
		t.Fatal("previous bytes", e)
	}
	if _, e := decodeSelection(selected.raw); e != nil {
		t.Fatal("selected bytes", e)
	}
	bodies["infrastructure-forwarding.json"] = []byte("new generation")
	bodies["forwarding-request.json"] = encode(selected.binding.Request)
	bodies["rendered-forwarding.service"], err = RenderUnit([]byte(Template), selected.binding.Request, selected.binding.Generation)
	if err != nil {
		t.Fatal(err)
	}
	bodies["binding.json"] = selected.raw
	bodies["root.json"] = []byte("root")
	bodies["snapshot.json"] = []byte("snapshot")
	bodies["timestamp.json"] = []byte("timestamp")
	hashes := map[string]string{}
	for name, raw := range bodies {
		hashes[name] = digest(raw)
	}
	sealed := filepath.Join(root, "generations", selected.binding.Generation)
	for i := range rows {
		r := &rows[i]
		r.Path = strings.ReplaceAll(strings.ReplaceAll(r.Path, oldRoot, root), oldHistory, history)
		switch r.Phase {
		case "replacement-admitted", "selection-intent", "selection-written", "replacement-verified":
			r.SHA256 = digest(selected.raw)
		case "predecessor-binding":
			r.SHA256 = digest(previous.raw)
		case "predecessor-unit":
			r.SHA256 = previous.binding.UnitSHA256
		case "unit-intent", "unit-written", "reload-intent":
			r.SHA256 = selected.binding.UnitSHA256
		case "file-intent", "file-ready":
			if r.Path == replacementPending(root) {
				r.SHA256 = digest(replacementPendingBytes(selected.raw))
			}
			if filepath.Dir(r.Path) == sealed {
				r.SHA256 = hashes[filepath.Base(r.Path)]
			}
		}
	}
	raw := encodeReplacementVector(t, rows)
	if _, e := decodeReplacementContextRecords(raw, selected, previous, hashes, before, after); e != nil {
		t.Fatal("detached grammar", e)
	}
	retirement := filepath.Join(attemptNumber(0).retirement(root), "retirement-result.json")
	journal := replacementJournal(root, selected.binding.Generation)
	write := func(path string, body []byte) {
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, body, 0600); e != nil {
			t.Fatal(e)
		}
	}
	write(journal, raw)
	write(journal+".native-before", before)
	write(journal+".native-after", after)
	write(retirement, []byte("retirement"))
	if err := os.Chown(filepath.Dir(retirement), 0, int(selected.binding.GID)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(retirement), 0750); err != nil {
		t.Fatal(err)
	}
	switch fault {
	case "root-parent-group":
		err = os.Chown(filepath.Dir(retirement), 0, 0)
	case "wrong-parent-group":
		err = os.Chown(filepath.Dir(retirement), 0, int(selected.binding.GID)+1)
	case "writable-parent":
		err = os.Chmod(filepath.Dir(retirement), 0770)
	case "wrong-parent-owner":
		err = os.Chown(filepath.Dir(retirement), int(selected.binding.UID), int(selected.binding.GID))
	case "wrong-leaf-group":
		err = os.Chown(retirement, 0, int(selected.binding.GID))
	case "symlink-leaf":
		err = os.Rename(retirement, retirement+".foreign")
		if err == nil {
			err = os.Symlink(retirement+".foreign", retirement)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &completedFiles{ctx: ctx, files: map[string]*initialFile{}, dirs: map[string]*inputDirectory{}}
	t.Cleanup(func() { _ = f.close() })
	for name, body := range bodies {
		f.files[filepath.Join(sealed, name)] = &initialFile{body: body}
	}
	reached := false
	f.observe = func(stage string, file *os.File) {
		if stage == "read" && f.files[retirement] != nil && file == f.files[retirement].file {
			reached = true
			cancel()
		}
	}
	c := &completedInitial{files: f, binding: selected.binding, selectionOrder: []generationSelection{selected, previous}, selections: map[string]generationSelection{digest(previous.raw): previous}}
	err = c.readReplacementHistory()
	// The original replacement flock remains exclusive through failure and join.
	peer, e := os.OpenFile(journal, os.O_RDONLY, 0)
	if e != nil {
		t.Fatal(e)
	}
	if e := syscall.Flock(int(peer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(e, syscall.EWOULDBLOCK) {
		_ = peer.Close()
		_ = f.close()
		t.Fatal("journal lease returned before close", e)
	}
	closeErr := f.close()
	if e := syscall.Flock(int(peer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		_ = peer.Close()
		t.Fatal("journal lease retained after physical close", e)
	}
	if e := peer.Close(); e != nil {
		t.Fatal(e)
	}
	if fault != "bound-parent" {
		if reached || err == nil || errors.Is(err, context.Canceled) || closeErr != nil {
			t.Fatalf("foreign retirement parent/leaf reached=%v error=%v close=%v", reached, err, closeErr)
		}
		return
	}
	if !reached || !errors.Is(err, context.Canceled) || !errors.Is(closeErr, context.Canceled) {
		t.Fatalf("retirement read reached=%v error=%v close=%v", reached, err, closeErr)
	}

	// A bound Node-group directory is admitted only by the specific retirement
	// reader. Ordinary input ancestors must retain the stricter root:root rule.
	strict := &completedFiles{ctx: context.Background(), files: map[string]*initialFile{}, dirs: map[string]*inputDirectory{}}
	_, strictErr := strict.read(retirement, 64<<10, 0600, 0, 0)
	if e := strict.close(); e != nil {
		t.Fatal(e)
	}
	if !errors.Is(strictErr, ErrBinding) {
		t.Fatal("ordinary ancestor policy widened", strictErr)
	}
}
