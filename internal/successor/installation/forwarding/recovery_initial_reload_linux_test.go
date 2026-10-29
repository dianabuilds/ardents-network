package forwarding

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Actual directory reads only, without Initial/Release or original-cut rights.
// A fresh read must detect a semantic history appearing after the earlier one.
func TestForwardingInitialReadbackRechecksEmptySemanticRoots(t *testing.T) {
	for changed := 0; changed < 4; changed++ {
		t.Run([]string{"state", "participation", "spend", "hosting"}[changed], func(t *testing.T) {
			base := t.TempDir()
			r := Request{StateRoot: filepath.Join(base, "state"), ParticipationRoot: filepath.Join(base, "participation"), SpendRoot: filepath.Join(base, "spend"), HostingRoot: filepath.Join(base, "hosting")}
			paths := []string{r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot}
			open := func() *completedFiles {
				t.Helper()
				files := &completedFiles{ctx: t.Context(), dirs: map[string]*inputDirectory{}}
				for _, path := range paths {
					file, err := os.Open(path)
					if err != nil {
						t.Fatal(err)
					}
					info, err := file.Stat()
					if err != nil {
						file.Close()
						t.Fatal(err)
					}
					dir := &inputDirectory{path: path, file: file, info: info}
					files.dirs[path] = dir
					files.dirOrder = append(files.dirOrder, dir)
				}
				return files
			}
			for _, path := range paths {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			original := open()
			if err := checkInitialEmptyRoots(original, r); err != nil {
				t.Fatal("original empty roots", err)
			}
			if err := original.close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(paths[changed], "unknown-history")
			if err := os.WriteFile(path, []byte("preserve unknown producer\n"), 0600); err != nil {
				t.Fatal(err)
			}
			fresh := open()
			defer fresh.close()
			if err := checkInitialEmptyRoots(fresh, r); err == nil {
				t.Fatal("fresh readback adopted unknown semantic history")
			}
			body, err := os.ReadFile(path)
			if err != nil || string(body) != "preserve unknown producer\n" {
				t.Fatal("refusal modified semantic history", err)
			}
		})
	}
}

// These complete literal original inventories and invented native identities
// exercise bytes only. No process cut, Reload, fresh authority or successful
// recovery is supplied by the vector.
func TestForwardingInitialGenerationPrefixesRetainOnlyOriginalArtifacts(t *testing.T) {
	rows, candidate, cut := recoveryInitialVector(t)
	b := candidate.selection.binding
	directory := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	// Literal original birth positions are independent of the parser's loop.
	artifacts := []struct {
		name string
		born int
	}{
		{"ardents-forwarding.service", 73},
		{"ardents-node-linux-amd64", 76},
		{"binding.json", 79},
		{"forwarding-plan.json", 82},
		{"forwarding-request.json", 85},
		{"infrastructure-enrollment.json", 88},
		{"infrastructure-forwarding.json", 91},
		{"rendered-forwarding.service", 94},
		{"root.json", 97},
		{"snapshot.json", 100},
		{"targets.json", 103},
		{"timestamp.json", 106},
	}
	for count := 72; count < 105; count++ {
		prefix := encodeReplacementVector(t, rows[:count])
		owned, err := decodeInitialPreReloadRecords(prefix, b, candidate.hashes)
		if err != nil {
			t.Fatal("original generation prefix", count, err)
		}
		wanted := 13 // Complete journal, blocker, history, eight dirs and two credentials.
		for _, artifact := range artifacts {
			path := filepath.Join(directory, artifact.name)
			actual, exists := owned[path]
			if count <= artifact.born {
				if exists {
					t.Fatal("absent or intent-only artifact invented custody", count, path)
				}
				continue
			}
			wanted++
			position := artifact.born
			if count > artifact.born+1 {
				position++
			}
			if !exists || actual != rows[position] {
				t.Fatal("original incomplete birth or ready provenance changed", count, path, actual)
			}
		}
		if len(owned) != wanted || owned[directory] != rows[50] || owned[directory].Mode != 0755 {
			t.Fatal("prefix invented readiness or sealing", count, len(owned), wanted)
		}
		for index := 72; index < count; index++ {
			changed := append([]initialRecord(nil), rows[:count]...)
			changed[index].Path = filepath.Join(directory, "foreign-artifact")
			if _, err := decodeInitialPreReloadRecords(encodeReplacementVector(t, changed), b, candidate.hashes); err == nil {
				t.Fatal("foreign original artifact record accepted", count, index)
			}
		}
		cut.Boundary.Records = uint32(count)
		cut.Boundary.PrefixSHA256 = digest(prefix)
		if _, err := initialPreReloadInventory(prefix, cut, candidate); err == nil {
			t.Fatal("byte classification granted unfinished native continuation", count)
		}
	}
}

func TestForwardingInitialPreReloadInventoryDoesNotInventCompletion(t *testing.T) {
	rows, candidate, cut := recoveryInitialVector(t)
	for count := 1; count <= len(rows); count++ {
		prefix := encodeReplacementVector(t, rows[:count])
		cut.Boundary.Records = uint32(count)
		cut.Boundary.PrefixSHA256 = digest(prefix)
		owned, err := initialPreReloadInventory(prefix, cut, candidate)
		if count < 105 || count > 118 {
			if err == nil {
				t.Fatal("unproved pre-Reload boundary", count)
			}
			continue
		}
		wanted := 27
		if count <= 115 {
			wanted = 26
			if _, exists := owned["/etc/systemd/system/ardents-forwarding.service"]; exists {
				t.Fatal("absent unit received invented original custody", count)
			}
		}
		if count <= 111 {
			wanted = 25
			if _, exists := owned[filepath.Join(candidate.selection.binding.Request.InstallationRoot, "binding.json")]; exists {
				t.Fatal("absent selection received invented original custody", count)
			}
		}
		if count <= 106 {
			wanted = 24
		}
		if err != nil || len(owned) != wanted {
			t.Fatal("complete original inventory unavailable", count, len(owned), err)
		}
		if count == 112 {
			birth := owned[filepath.Join(candidate.selection.binding.Request.InstallationRoot, "binding.json")]
			if birth.Phase != "file-born" || birth.Sequence != 111 || birth.Mode != 0600 || birth.SHA256 != "" {
				t.Fatal("empty selection acquired invented readiness", birth)
			}
		}
		if count <= 109 {
			b := candidate.selection.binding
			path := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
			dir := owned[path]
			if !validInitialSealDirectory(&dir, path) || dir.Mode != 0755 || dir.Phase != "directory-ready" {
				t.Fatal("unsealed directory acquired invented completion", count, dir)
			}
		}
		if _, err := decodeInitialRecords(prefix, candidate.selection.binding, candidate.hashes); err == nil {
			t.Fatal("prefix became a completed initial transaction", count)
		}
		if _, err := initialStoppedPrefix(prefix, cut, candidate.selection); err == nil {
			t.Fatal("pre-Reload bytes invented stopped verification", count)
		}
	}
	for n := range rows[:117] {
		bad := append([]initialRecord(nil), rows[:117]...)
		bad[n].Path += "-foreign"
		prefix := encodeReplacementVector(t, bad)
		cut.Boundary.Records = 117
		cut.Boundary.PrefixSHA256 = digest(prefix)
		if _, err := initialPreReloadInventory(prefix, cut, candidate); err == nil {
			t.Fatal("foreign original object admitted", n)
		}
	}
	prefix := encodeReplacementVector(t, rows[:118])
	cut.Boundary.Records = 118
	cut.Boundary.PrefixSHA256 = digest(prefix)
	for _, fault := range []string{"program", "manager", "context", "count", "digest", "binding", "journal"} {
		bad := cut
		switch fault {
		case "program":
			bad.Origin.ProgramSHA256 = digest([]byte("other program"))
		case "manager":
			bad.Boundary.Manager = "other manager"
		case "context":
			bad.Boundary.NativeContext = bytes.Replace(bad.Boundary.NativeContext, []byte(`"inode":101,`), []byte(`"inode":102,`), 1)
		case "count":
			bad.Boundary.Records--
		case "digest":
			bad.Boundary.PrefixSHA256 = digest([]byte("other prefix"))
		case "binding":
			bad.Boundary.Binding = []byte("other binding")
		case "journal":
			bad.Boundary.Journal += "-other"
		}
		if _, err := initialPreReloadInventory(prefix, bad, candidate); err == nil {
			t.Fatal("unproved original source ordering", fault)
		}
	}
}

// These byte vectors describe an absent or original empty timestamp, without
// granting its reconstruction, a native cut, Release or recovery permission.
func TestForwardingInitialTimestampPrefixPreservesIncompleteBirth(t *testing.T) {
	rows, candidate, cut := recoveryInitialVector(t)
	b := candidate.selection.binding
	sealed := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	path := filepath.Join(sealed, "timestamp.json")
	for _, count := range []int{105, 106, 107} {
		prefix := encodeReplacementVector(t, rows[:count])
		owned, err := decodeInitialPreReloadRecords(prefix, b, candidate.hashes)
		wanted := 24
		if count == 107 {
			wanted = 25
		}
		if err != nil || len(owned) != wanted {
			t.Fatal("incomplete timestamp inventory", count, len(owned), err)
		}
		directory := owned[sealed]
		if !validInitialSealDirectory(&directory, sealed) {
			t.Fatal("timestamp prefix invented generation sealing", count)
		}
		birth, present := owned[path]
		if count == 107 {
			if !present || birth != rows[106] || birth.Phase != "file-born" || birth.Mode != 0600 || birth.SHA256 != "" {
				t.Fatal("original empty timestamp acquired invented readiness", count, birth)
			}
		} else if present {
			t.Fatal("absent timestamp acquired invented custody", count)
		}
		for _, absent := range []string{filepath.Join(b.Request.InstallationRoot, "binding.json"), "/etc/systemd/system/ardents-forwarding.service"} {
			if _, exists := owned[absent]; exists {
				t.Fatal("later leaf acquired invented birth", count, absent)
			}
		}
		if _, err := decodeInitialRecords(prefix, b, candidate.hashes); err == nil {
			t.Fatal("incomplete timestamp became completed installation", count)
		}
		cut.Boundary.Records, cut.Boundary.PrefixSHA256 = uint32(count), digest(prefix)
		if classified, err := initialPreReloadInventory(prefix, cut, candidate); err != nil || len(classified) != wanted {
			t.Fatal("original timestamp inventory differs from codec", count, err)
		}
		for n := range rows[:count] {
			bad := append([]initialRecord(nil), rows[:count]...)
			bad[n].Path += "-foreign"
			if _, err := decodeInitialPreReloadRecords(encodeReplacementVector(t, bad), b, candidate.hashes); err == nil {
				t.Fatal("changed original timestamp prefix accepted", count, n)
			}
		}
	}
}

func TestForwardingInitialReloadCompletionCommitsOwnAfterContext(t *testing.T) {
	rows, candidate, cut := recoveryInitialVector(t)
	after := bytes.Replace(cut.Boundary.NativeContext, []byte(`"inode":101,`), []byte(`"inode":102,`), 1)
	if bytes.Equal(after, cut.Boundary.NativeContext) {
		t.Fatal("missing generator mutation")
	}
	for _, count := range []int{105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118} {
		prefix := encodeReplacementVector(t, rows[:count])
		cut.Boundary.Records = uint32(count)
		cut.Boundary.PrefixSHA256 = digest(prefix)
		original := &recoveryOriginal{cut: cut, prefix: &transitionPrefix{journal: &initialFile{body: bytes.Clone(prefix)}}, result: &initialFile{body: []byte("detached original-cut vector\n")}}
		var creation, selectionCreation, artifactCreation *initialRecord
		if count <= 115 {
			birth := rows[115]
			creation = &birth
		}
		if count <= 111 {
			birth := rows[111]
			selectionCreation = &birth
		}
		if count <= 106 {
			birth := rows[106]
			artifactCreation = &birth
		}
		if creation != nil {
			if _, err := initialReloadCompletion(original, candidate, after, nil, selectionCreation, artifactCreation); err == nil {
				t.Fatal("missing actual new unit birth accepted", count)
			}
		}
		if selectionCreation != nil {
			if _, err := initialReloadCompletion(original, candidate, after, creation, nil, artifactCreation); err == nil {
				t.Fatal("missing actual selection creation accepted", count)
			}
		}
		if artifactCreation != nil {
			if _, err := initialReloadCompletion(original, candidate, after, creation, selectionCreation, nil); err == nil {
				t.Fatal("missing actual timestamp creation accepted", count)
			}
		}
		raw, err := initialReloadCompletion(original, candidate, after, creation, selectionCreation, artifactCreation)
		if err != nil {
			t.Fatal(count, err)
		}
		completion, full, err := decodeRecoveryCompletion(raw, prefix, original.result.body, cut)
		if err != nil || completion.Schema != "ardents-forwarding-recovery-completion-v2" || completion.NativeContextSHA256 != digest(after) || len(completion.Records) != 120-count || !bytes.Equal(full, encodeReplacementVector(t, rows)) || !bytes.Equal(prefix, original.prefix.journal.body) {
			t.Fatal("separate original/recovery commitments", count, err)
		}
		for _, fault := range []string{"missing-context", "v1-field", "v2-start", "late-cut"} {
			bad, originalCut := completion, cut
			switch fault {
			case "missing-context":
				bad.NativeContextSHA256 = ""
			case "v1-field":
				bad.Schema = "ardents-forwarding-recovery-completion-v1"
			case "v2-start":
				bad.Operation = "start"
			case "late-cut":
				originalCut.Boundary.Records = 119
			}
			encoded, _ := json.Marshal(bad)
			if _, _, err := decodeRecoveryCompletion(append(encoded, '\n'), prefix, original.result.body, originalCut); err == nil {
				t.Fatal("unversioned or foreign context proof", fault)
			}
		}
		changed := bytes.Replace(after, []byte(`"started":1`), []byte(`"started":2`), 1)
		if bytes.Equal(changed, after) {
			t.Fatal("missing context control")
		}
		if _, err := initialReloadCompletion(original, candidate, changed, creation, selectionCreation, artifactCreation); err == nil {
			t.Fatal("unrelated native identity changed through Reload")
		}
	}
}
